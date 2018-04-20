package stub

const (
	prometheusJmxExportedConfigFilename           = "config.yaml"
	prometheusJmxExporterTargetDir                = "/opt/jmx-exporter-loader"
	prometheusJmxExporterTargetConfDir            = "conf"
	prometheusJmxExporterSrcJarsDir               = "/opt/jmx-exporter-loader"
	prometheusJmxExporterAnnotationKey            = "jmx-prometheus-exporter"
	prometheusJmxExporterAnnotationVerified       = "verified"
	prometheusJmxExporterAnnotationVerifiedFailed = "verified-failed"
	prometheusJmxExporterLoaderJar                = "jmx-exporter-loader-1.0.jar"
	prometheusJmxExporterAgentJar                 = "jmx_prometheus_javaagent-0.3.1.jar"
	prometheusJmxExporterLoaderClass              = "com.banzaicloud.JmxExporterLoader"
)
