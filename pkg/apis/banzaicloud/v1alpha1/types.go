package v1alpha1

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type PrometheusJmxExporterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []PrometheusJmxExporter `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type PrometheusJmxExporter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              PrometheusJmxExporterSpec   `json:"spec"`
	Status            PrometheusJmxExporterStatus `json:"status,omitempty"`
}

type PrometheusJmxExporterSpec struct {
	LabelSelector map[string]string `json:"labelSelector,required"`
	Config        struct {
		ConfigMapName string `json:"configMapName,required"`
		ConfigMapKey  string `json:"configMapKey,required"`
	} `json:"config"`
	Port int `json: port,required`
}

type PrometheusJmxExporterConfig struct {
	StartDelaySeconds         *int                               `yaml:"startDelaySeconds,omitempty"`
	HostPort                  *string                            `yaml:"hostPort,omitempty"`
	Username                  *string                            `yaml:"username,omitempty"`
	Password                  *string                            `yaml:"password,omitempty"`
	JmxUrl                    *string                            `yaml:"jmxUrl,omitempty"`
	Ssl                       *bool                              `yaml:"ssl,omitempty"`
	LowercaseOutputName       *bool                              `yaml:"lowercaseOutputName,omitempty"`
	LowercaseOutputLabelNames *bool                              `yaml:"lowercaseOutputLabelNames,omitempty"`
	WhitelistObjectNames      []string                           `yaml:"whitelistObjectNames,omitempty"`
	BlacklistObjectNames      []string                           `yaml:"blacklistObjectNames,omitempty"`
	Rules                     []PrometheusJmxExporterConfigRules `yaml:"rules,omitempty"`
}

type PrometheusJmxExporterConfigRules struct {
	Pattern           *string           `yaml:"pattern,omitempty"`
	Name              *string           `yaml:"name,omitempty"`
	Value             *string           `yaml:"value,omitempty"`
	ValueFactor       *float32          `yaml:"valueFactor,omitempty"`
	Labels            map[string]string `yaml:"labels,omitempty"`
	Help              *string           `yaml:"help,omitempty"`
	Type              *string           `yaml:"type,omitempty"`
	AttrNameSnakeCase *bool             `yaml:"attrNameSnakeCase,omitempty"`
}

type PrometheusJmxExporterStatus struct {
	MetricsEndpoints []*MetricsEndpoint `json: metricsEndpoints,omitempty`
}

type MetricsEndpoint struct {
	Pod  string `json:"pod,required"`
	Port int    `json:"port,required"`
}

// equals returns true if a equals b otherwise false
func (this PrometheusJmxExporterStatus) Equals(that PrometheusJmxExporterStatus) bool {
	if len(this.MetricsEndpoints) != len(that.MetricsEndpoints) {
		return false
	}

	diff := make(map[string]int)
	for _, x := range this.MetricsEndpoints {
		key := fmt.Sprintf("%s:%d", x.Pod, x.Port)
		diff[key]++
	}

	for _, y := range that.MetricsEndpoints {
		key := fmt.Sprintf("%s:%d", y.Pod, y.Port)
		if _, ok := diff[key]; !ok {
			return false
		}

		diff[key]--
		if diff[key] == 0 {
			delete(diff, key)
		}
	}
	if len(diff) == 0 {
		return true
	}

	return false
}
