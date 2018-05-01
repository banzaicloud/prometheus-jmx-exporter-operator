package stub

import (
	"bytes"
	"fmt"
	"github.com/banzaicloud/prometheus-jmx-exporter-operator/pkg/apis/banzaicloud/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/sdk/action"
	"github.com/operator-framework/operator-sdk/pkg/sdk/handler"
	"github.com/operator-framework/operator-sdk/pkg/sdk/query"
	"github.com/operator-framework/operator-sdk/pkg/sdk/types"
	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"os"
	"path"
	"strconv"
	"strings"
)

func NewHandler() handler.Handler {
	return &Handler{}
}

type Handler struct {
	// Fill me
}

func (h *Handler) Handle(ctx types.Context, event types.Event) error {
	switch o := event.Object.(type) {
	case *v1alpha1.PrometheusJmxExporter:
		prometheusJmxExporter := o

		logrus.Infof("PrometheusJmxExporter event received for '%s/%s'",
			prometheusJmxExporter.Namespace,
			prometheusJmxExporter.Name)

		if event.Deleted {
			logrus.Infof(
				"PrometheusJmxExporter deleted event received for '%s/%s'",
				prometheusJmxExporter.Namespace,
				prometheusJmxExporter.Name)

			return nil
		}

		logrus.Infof(
			"Retrieving prometheus jmx exporter config from configMap '%s/%s:%s'",
			prometheusJmxExporter.Namespace,
			prometheusJmxExporter.Spec.Config.ConfigMapName,
			prometheusJmxExporter.Spec.Config.ConfigMapKey)

		config, err := getConfig(
			prometheusJmxExporter.Namespace,
			prometheusJmxExporter.Spec.Config.ConfigMapName,
			prometheusJmxExporter.Spec.Config.ConfigMapKey)

		logrus.Debug(config)

		if err != nil {
			logrus.Errorf("Error during retrieving prometheus jmx exporter config")
			return err
		}

		logrus.Infof("Retrieving pods with label: '%s'", prometheusJmxExporter.Spec.LabelSelector)
		podList, err := queryPods(
			prometheusJmxExporter.Namespace,
			labels.SelectorFromSet(prometheusJmxExporter.Spec.LabelSelector).String())
		if err != nil {
			logrus.Errorf("Error during querying pods : %v", err)
			return err
		}

		logrus.Infof("Pods found: namespace='%s', %s", prometheusJmxExporter.Namespace, formatSimplePods(podList.Items))

		if len(podList.Items) == 0 {
			return nil
		}

		if err := checkPrometheusJmxExporterConflict(podList, prometheusJmxExporter); err != nil {
			return err
		}

		processPods(podList.Items, config, prometheusJmxExporter.Spec.Port)

		// update status
		newStatus := createPrometheusJmxExporterStatus(podList.Items)

		if !prometheusJmxExporter.Status.Equals(newStatus) {
			prometheusJmxExporter.Status = createPrometheusJmxExporterStatus(podList.Items)

			logrus.Infof(
				"PrometheusJmxExporter: '%s/%s' : Update status",
				prometheusJmxExporter.Namespace,
				prometheusJmxExporter.Name)

			action.Update(prometheusJmxExporter)
		}

	case *v1.Pod:
		pod := o

		if !event.Deleted && pod.Status.Phase != v1.PodRunning {
			// process only running pods
			return nil
		}

		prometheusJmxExporters, err := queryPrometheusJmxExporters(pod.Namespace)
		if err != nil {
			logrus.Errorf("Error during querying prometheusjmxexporters in namespace '%s': %v", pod.Namespace, err)
			return err
		}

		prometheusJmxExporter, err := findExporterForPod(prometheusJmxExporters, pod)
		if err != nil {
			return err
		}
		if prometheusJmxExporter == nil {
			return nil
		}

		if event.Deleted {
			logrus.Infof("Pod deleted event received for '%s/%s'", pod.Namespace, pod.Name)
			// pod is being deleted thus remove the prometheus endpoint that
			// is exposed by this pod if there is any
			removePrometheusJmxExporterEndpoint(prometheusJmxExporter, pod)
			action.Update(prometheusJmxExporter)
			return nil
		}

		if isVerified(pod) {
			logrus.Infof("Ignoring pod '%s/%s' as it has already been processed.", pod.Namespace, pod.Name)
		} else {

			logrus.Infof("Retrieving prometheus jmx exporter config from configMap '%s/%s:%s'",
				prometheusJmxExporter.Namespace,
				prometheusJmxExporter.Spec.Config.ConfigMapName,
				prometheusJmxExporter.Spec.Config.ConfigMapKey)

			config, err := getConfig(
				prometheusJmxExporter.Namespace,
				prometheusJmxExporter.Spec.Config.ConfigMapName,
				prometheusJmxExporter.Spec.Config.ConfigMapKey)

			logrus.Debug(config)
			if err != nil {
				logrus.Errorf("Error during retrieving prometheus jmx exporter config")
				return err
			}

			if err := processPod(pod, config, prometheusJmxExporter.Spec.Port); err != nil {
				return err
			}

			if updatePrometheusJmxExporterEndpoints(prometheusJmxExporter, pod) {
				logrus.Infof(
					"PrometheusJmxExporter: '%s/%s' : Update status",
					prometheusJmxExporter.Namespace,
					prometheusJmxExporter.Name)

				action.Update(prometheusJmxExporter)
			}
		}
	}
	return nil
}

// queryPrometheusJmxExporters returns PrometheusJmxExporterList from given namespace
func queryPrometheusJmxExporters(namespace string) (*v1alpha1.PrometheusJmxExporterList, error) {
	jmxExporterList := v1alpha1.PrometheusJmxExporterList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PrometheusJmxExporter",
			APIVersion: "banzaicloud.com/v1alpha1",
		},
	}

	listOptions := query.WithListOptions(&metav1.ListOptions{
		IncludeUninitialized: false,
	})

	if err := query.List(namespace, &jmxExporterList, listOptions); err != nil {
		logrus.Errorf("Failed to query prometheusjmxexporters : %v", err)
		return nil, err
	}

	return &jmxExporterList, nil
}

// getConfig returns the content of the config data stored under configMapKey
func getConfig(namespace, configMapName, configMapKey string) (*v1alpha1.PrometheusJmxExporterConfig, error) {
	configMap := v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
		},
	}

	getOptions := query.WithGetOptions(&metav1.GetOptions{
		IncludeUninitialized: false,
	})

	err := query.Get(&configMap, getOptions)
	if err != nil {
		logrus.Errorf("Failed to get configMap namespace='%s', name='%s': %v", namespace, configMapName, err)
		return nil, err
	}

	config, ok := configMap.Data[configMapKey]
	if !ok {
		return nil, fmt.Errorf("configMap data with key '%s' not found in configMap '%s/%s'", configMapKey, namespace, configMapName)
	}

	logrus.Debugf("Validating config data '%s'", config)

	var configObj = v1alpha1.PrometheusJmxExporterConfig{}
	err = yaml.Unmarshal([]byte(config), &configObj)
	if err != nil {
		return nil, err
	}

	return &configObj, err
}

// queryPods returns list of pods according to the labelSelector
func queryPods(namespace, labelSelector string) (*v1.PodList, error) {
	podList := v1.PodList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
	}
	listOptions := query.WithListOptions(&metav1.ListOptions{
		LabelSelector:        labelSelector,
		IncludeUninitialized: false,
	})

	err := query.List(namespace, &podList, listOptions)
	if err != nil {
		logrus.Errorf("Failed to query pods : %v", err)
		return nil, err
	}

	var filteredPods []v1.Pod

	for i := 0; i < len(podList.Items); i++ {
		if podList.Items[i].Status.Phase == v1.PodRunning {
			filteredPods = append(filteredPods, podList.Items[i])
		}
	}

	podList.Items = filteredPods

	return &podList, nil

}

// processPods loads prometheus jmx exporter agent into each pod
func processPods(pods []v1.Pod, config *v1alpha1.PrometheusJmxExporterConfig, portNumber int) {
	logrus.Info("Processing running pods...")

	for i := 0; i < len(pods); i++ {
		pod := &pods[i]

		if isVerified(pod) {
			logrus.Infof("Ignoring pod '%s/%s' as it has already been processed.", pod.Namespace, pod.Name)
		} else {
			err := processPod(pod, config, portNumber)
			if err != nil {
				logrus.Warnf("Processing pod failed: %v", err)
			}
		}
	}

	logrus.Info("Processing pods finished.")
}

// isVerified returns true of the pod was already processed and verified
func isVerified(pod *v1.Pod) bool {
	v, ok := pod.Annotations[prometheusJmxExporterAnnotationKey]

	return ok && (v == prometheusJmxExporterAnnotationVerified || v == prometheusJmxExporterAnnotationVerifiedFailed)
}

// processPod loads prometheus jmx exporter agent into the pod
func processPod(pod *v1.Pod, config *v1alpha1.PrometheusJmxExporterConfig, portNumber int) error {
	logrus.Infof("Inspecting pod '%s'", pod.Name)

	if len(pod.Spec.Containers) > 0 {
		// TODO: prometheus doesn't support scraping multiple containers running on the same pod
		// TODO: in case of multiple containers which container we should go with
		container := pod.Spec.Containers[0]

		pids, err := queryJavaProcesses(pod, &container)

		if err != nil {
			logrus.Infof("Mark pod '%s' as verify failed", pod.Name)

			podVerifiedFailed(pod)
			return err
		}

		if len(pids) > 1 {
			// TODO: should we support multiple java processes per container?
			return fmt.Errorf("multiple java processes found")
		}

		// copy jars
		if err := copyJmxPrometheusExporterJars(pod, &container); err != nil {
			return err
		}

		// copy config to pod container
		if err := copyPrometheusJmxExporterConfToPod(config, pod, &container); err != nil {
			return err
		}

		// open port for prometheus jmx exporter

		// TODO: port number should be determined dynamically such way that doesn't conflicts with ports
		// already in use by the processes running in the container

		logrus.Infof("Exposing port number %d on '%s/%s/%s'", portNumber, pod.Namespace, pod.Name, container.Name)

		if err := exposeContainerPort(int32(portNumber), pod, &container); err != nil {
			return err
		}

		// load prometheus jmx exporter agent
		if err := loadPrometheusJmxExporterAgent(pod, &container, portNumber, pids[0]); err != nil {
			return err
		}

		// annotade pod for prometheus
		annotateForPrometheus(pod, portNumber)
	}

	logrus.Infof("Mark pod '%s' as verified", pod.Name)

	return podVerified(pod)

}

// podVerified updates the pod annotations to mark it as verified.
func podVerifiedFailed(pod *v1.Pod) error {
	annotations := map[string]string{
		prometheusJmxExporterAnnotationKey: prometheusJmxExporterAnnotationVerifiedFailed,
	}
	return annotatePod(pod, annotations)
}

// podVerified updates the pod annotations to mark it as verified.
func podVerified(pod *v1.Pod) error {
	annotations := map[string]string{
		prometheusJmxExporterAnnotationKey: prometheusJmxExporterAnnotationVerified,
	}

	return annotatePod(pod, annotations)
}

// annotatePod annotates pod with the given annotations
func annotatePod(pod *v1.Pod, annotations map[string]string) error {
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}

	for key, value := range annotations {
		pod.Annotations[key] = value
	}

	err := action.Update(pod)
	if err != nil {
		logrus.Errorf("Updating pod '%s' failed: %v", pod.Name, err)
		return err
	}

	return nil
}

// queryJavaProcesses inspects container for running java processes
func queryJavaProcesses(pod *v1.Pod, container *v1.Container) ([]string, error) {
	logrus.Infof("Inspecting container '%s' for java processes", container.Name)

	stdout, err := execCommand(pod.Namespace, pod.Name, nil, container,
		"sh", "-c", "$JAVA_HOME/bin/jps")

	if err != nil {
		logrus.Warnf("Failed to retrieve java process list: %v", err)

		return nil, err
	} else {
		var javaProcIds []string

		procs := strings.Split(strings.TrimSpace(stdout), "\n")
		for _, proc := range procs {
			idAndClassName := strings.Split(proc, " ")

			procId := idAndClassName[0]
			className := idAndClassName[1]

			if !strings.EqualFold(className, "jps") { // skip jps
				javaProcIds = append(javaProcIds, procId)
			}
		}

		logrus.Infof("Java processes: %s", javaProcIds)

		return javaProcIds, nil
	}
}

// copyPrometheusJmxExporterConfToPod creates  config file from the configContent and copies it to the pod container
func copyPrometheusJmxExporterConfToPod(configContent *v1alpha1.PrometheusJmxExporterConfig, pod *v1.Pod, container *v1.Container) error {
	tmpDir, err := ioutil.TempDir("", "prometheus-jmx-exporter-conf")
	if err != nil {
		return err
	}

	defer os.RemoveAll(tmpDir) // clean up

	configData, err := yaml.Marshal(configContent)
	if err != nil {
		return err
	}

	f, err := os.Create(path.Join(tmpDir, prometheusJmxExportedConfigFilename))
	if err != nil {
		return err
	}

	if _, err = f.Write(configData); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	err = copyToPod(pod.Namespace, pod.Name, container, tmpDir, path.Join(prometheusJmxExporterTargetDir, prometheusJmxExporterTargetConfDir))
	if err != nil {
		logrus.Errorf("Copying config for jmx-exporter to container '%s/%s/%s'failed: %v",
			pod.Namespace, pod.Name, container.Name, err)
		return err
	}

	return nil
}

// copyJmxPrometheusExporterJars copies the jars of prometheus jmx exporter to pod
func copyJmxPrometheusExporterJars(pod *v1.Pod, container *v1.Container) error {
	err := copyToPod(pod.Namespace, pod.Name, container, prometheusJmxExporterSrcJarsDir, prometheusJmxExporterTargetDir)
	if err != nil {
		logrus.Errorf("Copying jmx-exporter-loader jars to container '%s/%s/%s'failed: %v",
			pod.Namespace, pod.Name, container.Name, err)
		return err
	}

	return nil
}

// exposeContainerPort exposes portNumber on container
func exposeContainerPort(portNumber int32, pod *v1.Pod, container *v1.Container) error {
	for _, port := range container.Ports {
		if port.ContainerPort == portNumber {
			return fmt.Errorf("port number %d is already in use", portNumber)
		}
	}

	container.Ports = append(container.Ports, v1.ContainerPort{
		ContainerPort: portNumber,
		Name:          "prometheusJmxMetrics",
		Protocol:      v1.ProtocolTCP,
	})

	return action.Update(pod)
}

// loadPrometheusJmxExporterAgent loads prometheus jmx exporter agent into the process with pid
// running inside container
func loadPrometheusJmxExporterAgent(pod *v1.Pod, container *v1.Container, portNumber int, pid string) error {
	logrus.Infof("Loading prometheus jmx exporter agent into process with pid %s running inside '%s/%s/%s'",
		pid, pod.Namespace, pod.Name, container.Name)

	var javaCmd bytes.Buffer
	javaCmd.WriteString("$JAVA_HOME/bin/java -cp ")
	javaCmd.WriteString(path.Join(prometheusJmxExporterTargetDir, prometheusJmxExporterLoaderJar))
	javaCmd.WriteString(" -Dpid=")
	javaCmd.WriteString(pid)
	javaCmd.WriteString(" -Dprometheus.javaagent.path=")
	javaCmd.WriteString(path.Join(prometheusJmxExporterTargetDir, prometheusJmxExporterAgentJar))
	javaCmd.WriteString(" -Dprometheus.port=")
	javaCmd.WriteString(strconv.Itoa(portNumber))
	javaCmd.WriteString(" -Dprometheus.javaagent.configPath=")
	javaCmd.WriteString(path.Join(prometheusJmxExporterTargetDir, prometheusJmxExporterTargetConfDir, prometheusJmxExportedConfigFilename))
	javaCmd.WriteString(" ")
	javaCmd.WriteString(prometheusJmxExporterLoaderClass)

	command := []string{"sh", "-c", javaCmd.String()}

	_, err := execCommand(pod.Namespace, pod.Name, nil, container, command...)
	return err
}

// annotateForPrometheus places annotation on the pod that provide
func annotateForPrometheus(pod *v1.Pod, portNumber int) error {
	annotations := map[string]string{
		"prometheus.io/scrape": "true",
		"prometheus.io/port":   strconv.Itoa(portNumber),
	}

	return annotatePod(pod, annotations)
}

// createPrometheusJmxExporterStatus collects the endpoints through which prometheus can
// scrape metrics published by the prometheus jmx exporter
func createPrometheusJmxExporterStatus(pods []v1.Pod) v1alpha1.PrometheusJmxExporterStatus {
	var status v1alpha1.PrometheusJmxExporterStatus

	for i := 0; i < len(pods); i++ {
		pod := pods[i]

		if endpoint := createMetricEndpoint(&pod); endpoint != nil {
			status.MetricsEndpoints = append(status.MetricsEndpoints, endpoint)
		}
	}

	return status
}

// updatePrometheusJmxExporterEndpoints adds prometheus endpoint published by pod
// to the metrics endpoints of prometheusJmxExporter. If the pod doesn't exposes port for prometheus
// or port hasn't changed then metrics endpoints of prometheusJmxExporter is not changed.
// Returns true if metrics endpoints of prometheusJmxExporter is changed otherwise returns false.
func updatePrometheusJmxExporterEndpoints(prometheusJmxExporter *v1alpha1.PrometheusJmxExporter, pod *v1.Pod) bool {
	if endpoint := createMetricEndpoint(pod); endpoint != nil {
		for _, endpointUpd := range prometheusJmxExporter.Status.MetricsEndpoints {
			if endpointUpd.Pod == pod.Name {

				if endpointUpd.Port != endpoint.Port {
					endpointUpd.Port = endpoint.Port

					return true
				}

				return false
			}
		}

		prometheusJmxExporter.Status.MetricsEndpoints = append(prometheusJmxExporter.Status.MetricsEndpoints, endpoint)
		return true
	}

	return false
}

// removePrometheusJmxExporterEndpoint removes the endpoint entry from prometheusJmxExporter.Status.MetricsEndpoints
// that corresponds to pod
func removePrometheusJmxExporterEndpoint(prometheusJmxExporter *v1alpha1.PrometheusJmxExporter, pod *v1.Pod) {
	matchIdx := -1
	for i, endpointUpd := range prometheusJmxExporter.Status.MetricsEndpoints {
		if endpointUpd.Pod == pod.Name {
			matchIdx = i
			break
		}
	}
	if matchIdx == 0 {
		prometheusJmxExporter.Status.MetricsEndpoints = prometheusJmxExporter.Status.MetricsEndpoints[1:]
	} else if matchIdx > 0 {
		prometheusJmxExporter.Status.MetricsEndpoints =
			append(prometheusJmxExporter.Status.MetricsEndpoints[:matchIdx], prometheusJmxExporter.Status.MetricsEndpoints[matchIdx+1:]...)
	}
}

func createMetricEndpoint(pod *v1.Pod) *v1alpha1.MetricsEndpoint {
	if enabled, ok := pod.Annotations["prometheus.io/scrape"]; ok && enabled == "true" {
		if portStr, ok := pod.Annotations["prometheus.io/port"]; ok {
			port, _ := strconv.Atoi(portStr)

			return &v1alpha1.MetricsEndpoint{
				Pod:  pod.Name,
				Port: port,
			}
		}
	}
	return nil
}

// findExporterForPod searches through prometheusJmxExporterList and returns PrometheusJmxExporter
// of which  pod label selector matches the labels of pod. In case of multiple matches found return with error.
func findExporterForPod(prometheusJmxExporterList *v1alpha1.PrometheusJmxExporterList, pod *v1.Pod) (*v1alpha1.PrometheusJmxExporter, error) {
	var matchIdx []int

	for i := 0; i < len(prometheusJmxExporterList.Items); i++ {
		prometheusJmxExporter := prometheusJmxExporterList.Items[i]

		selector := labels.SelectorFromSet(prometheusJmxExporter.Spec.LabelSelector)
		if selector.Matches(labels.Set(pod.Labels)) {
			matchIdx = append(matchIdx, i)
		}
	}

	if len(matchIdx) > 1 {
		var list []v1alpha1.PrometheusJmxExporter
		for _, idx := range matchIdx {
			list = append(list, prometheusJmxExporterList.Items[idx])
		}
		logrus.Errorf("Multiple prometheusjmxexporters for pod '%s/%s' found: %s",
			pod.Namespace, pod.Name, formatSimplePrometheusJmxExporters(list))

		return nil, fmt.Errorf("multiple prometheusjmxexporters for pod '%s/%s' found", pod.Namespace, pod.Namespace)
	}

	if len(matchIdx) == 0 {
		return nil, nil
	}

	return &prometheusJmxExporterList.Items[matchIdx[0]], nil
}

// checkPrometheusJmxExporterConflict verifies whether beside prometheusJmxExporter there are any other
// PrometheusJmxExporters that matches the pods in podList
func checkPrometheusJmxExporterConflict(podList *v1.PodList, prometheusJmxExporter *v1alpha1.PrometheusJmxExporter) error {
	prometheusJmxExporters, err := queryPrometheusJmxExporters(prometheusJmxExporter.Namespace)
	if err != nil {
		return err
	}

	for i := 0; i < len(podList.Items); i++ {
		pod := podList.Items[i]

		for j := 0; j < len(prometheusJmxExporters.Items); j++ {
			otherPrometheusJmxExporter := prometheusJmxExporters.Items[j]

			if otherPrometheusJmxExporter.Name != prometheusJmxExporter.Name {
				selector := labels.SelectorFromSet(otherPrometheusJmxExporter.Spec.LabelSelector)

				if selector.Matches(labels.Set(pod.Labels)) {

					logrus.Errorf("prometheusjmxexporter '%s' for pod '%s/%s' already defined",
						pod.Namespace,
						pod.Namespace,
						otherPrometheusJmxExporter.Name)

					return fmt.Errorf(
						"prometheusjmxexporter '%s' for pod '%s/%s' already defined",
						pod.Namespace,
						pod.Namespace,
						otherPrometheusJmxExporter.Name)
				}
			}

		}
	}

	return nil
}

// formatSimplePods returns the name of the pods delimited by comma surrounded by parenthesis.
func formatSimplePods(pods []v1.Pod) string {
	var buffer bytes.Buffer
	buffer.WriteString("(")
	for i := 0; i < len(pods); i++ {
		pod := pods[i]

		if i != 0 {
			buffer.WriteString(",")
		}
		buffer.WriteString(pod.Name)
	}
	buffer.WriteString(")")

	return buffer.String()
}

// formatSimplePrometheusJmxExporters returns the name of the prometheusjmxexporters delimited by comma surrounded by parenthesis.
func formatSimplePrometheusJmxExporters(prometheusjmxexporters []v1alpha1.PrometheusJmxExporter) string {
	var buffer bytes.Buffer
	buffer.WriteString("(")
	for i := 0; i < len(prometheusjmxexporters); i++ {
		prometheusjmxexporter := prometheusjmxexporters[i]

		if i != 0 {
			buffer.WriteString(",")
		}
		buffer.WriteString(prometheusjmxexporter.Name)
	}
	buffer.WriteString(")")

	return buffer.String()
}
