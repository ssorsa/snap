package influx

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/intelsdi-x/pulse/control/plugin"
	"github.com/intelsdi-x/pulse/control/plugin/cpolicy"
	"github.com/intelsdi-x/pulse/core/ctypes"

	"github.com/influxdb/influxdb/client"
)

const (
	name       = "influx"
	version    = 1
	pluginType = plugin.PublisherPluginType
)

// Meta returns a plugin meta data
func Meta() *plugin.PluginMeta {
	return plugin.NewPluginMeta(name, version, pluginType, []string{plugin.PulseGOBContentType}, []string{plugin.PulseGOBContentType})
}

//NewInfluxPublisher returns an instance of the InfuxDB publisher
func NewInfluxPublisher() *influxPublisher {
	return &influxPublisher{}
}

type influxPublisher struct {
}

func (f *influxPublisher) GetConfigPolicyNode() cpolicy.ConfigPolicyNode {
	config := cpolicy.NewPolicyNode()

	r1, err := cpolicy.NewStringRule("host", true)
	handleErr(err)
	r1.Description = "Influxdb host"
	config.Add(r1)

	r2, err := cpolicy.NewIntegerRule("port", true)
	handleErr(err)
	r2.Description = "Influxdb port"
	config.Add(r2)

	r3, err := cpolicy.NewStringRule("database", true)
	handleErr(err)
	r3.Description = "Influxdb db name"
	config.Add(r3)

	r4, err := cpolicy.NewStringRule("user", true)
	handleErr(err)
	r4.Description = "Influxdb user"
	config.Add(r4)

	r5, err := cpolicy.NewStringRule("password", true)
	handleErr(err)
	r5.Description = "Influxdb password"
	config.Add(r4)

	return *config
}

// Publish publishes metric data to influxdb
// currently only 0.9 version of influxdb are supported
func (f *influxPublisher) Publish(contentType string, content []byte, config map[string]ctypes.ConfigValue, logger *log.Logger) error {
	logger.Println("Publishing started")
	var metrics []plugin.PluginMetricType

	switch contentType {
	case plugin.PulseGOBContentType:
		dec := gob.NewDecoder(bytes.NewBuffer(content))
		if err := dec.Decode(&metrics); err != nil {
			logger.Printf("Error decoding: error=%v content=%v", err, content)
			return err
		}
	default:
		logger.Printf("Error unknown content type '%v'", contentType)
		return fmt.Errorf("Unknown content type '%s'", contentType)
	}

	u, err := url.Parse(fmt.Sprintf("http://%s:%d", config["host"].(ctypes.ConfigValueStr).Value, config["port"].(ctypes.ConfigValueInt).Value))
	if err != nil {
		logger.Fatal(err)
	}

	conf := client.Config{
		URL:       *u,
		Username:  config["user"].(ctypes.ConfigValueStr).Value,
		Password:  config["password"].(ctypes.ConfigValueStr).Value,
		UserAgent: "pulse-publisher",
	}

	con, err := client.NewClient(conf)
	if err != nil {
		logger.Fatal(err)
	}

	dur, ver, err := con.Ping()
	if err != nil {
		logger.Printf("ERROR publishing %v to %v with %v %v", metrics, config, ver, dur)
		logger.Fatal(err)
	}

	pts := make([]client.Point, len(metrics))
	var value string
	for i, m := range metrics {
		switch v := m.Data().(type) {
		case int:
			value = strconv.Itoa(v)
		case uint64:
			value = strconv.FormatUint(v, 10)
		case float64:
			value = strconv.FormatFloat(v, 'f', 6, 64)
		default:
			return fmt.Errorf("Unsupported data type '%T'", v)
		}
		pts[i] = client.Point{
			Name: strings.Join(m.Namespace(), "/"),
			Fields: map[string]interface{}{
				"value": value,
			},
		}
	}

	bps := client.BatchPoints{
		Timestamp:       time.Now(),
		Precision:       "s",
		Points:          pts,
		Database:        config["database"].(ctypes.ConfigValueStr).Value,
		RetentionPolicy: "default",
	}

	_, err = con.Write(bps)
	if err != nil {
		logger.Fatal(err)
	}

	return nil
}

func handleErr(e error) {
	if e != nil {
		panic(e)
	}
}
