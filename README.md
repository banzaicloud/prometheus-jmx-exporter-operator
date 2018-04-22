# Prometheus Jmx Exporter Operator

This operator using [Jmx Exporter](https://github.com/prometheus/jmx_exporter) enables Java processes running ok Kubernetes Pods to expose metrics collected form mBeans via JMX to Prometheus.

## Usage

### Prometheus JMX Exporter Configuration

#### Create a [configuration](https://github.com/prometheus/jmx_exporter#configuration) file for the JMX Exporter. 
*Note* that in our case `hostPort` and `jmxUrl` should not be set as the Jmx Exporter agent will be loaded into the Java process and we want it to talk to the local JVM. More configuration examples of Prometheus JMX Exporter can be found [here](https://github.com/prometheus/jmx_exporter/tree/master/example_configs)
#### Add to configuration to a Kubernetes [config map](https://kubernetes.io/docs/tasks/configure-pod-container/configure-pod-configmap/)
```kubectl create cm <your-prometheus-jmx-exporter-configmap-name> --from-file=<path-to-the-config-file>```

e.g.:
 ```sh
kubectl create cm prometheus-jmx-exporter-config --from-file=javadummy/config.yaml

kubectl describe cm prometheus-jmx-exporter-config
```  
#### RBAC
If [RBAC](https://kubernetes.io/docs/admin/authorization/rbac/) is enabled in your Kubernetes cluster than a [service account](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/) is needed to be created with the approprite role binding for the operator.
Download the [rbac.yaml](https://github.com/banzaicloud/prometheus-jmx-exporter-operator/blob/master/deploy/rbac.yaml) file then execute:

```sh 
kubectl create -f <path-to-rbac-yaml-file>
```

#### Deploy the operator to Kubernetes
Download the [operator.yaml](https://github.com/banzaicloud/prometheus-jmx-exporter-operator/blob/master/deploy/operator.yaml) deployment file. If you don't have RBAC enabled than remove
```
serviceAccountName: prometheus-jmx-exporter-operator
```


```sh
kubectl create -f <path-to-your-operator-yaml-file>
```
#### Create `prometheus-jmx-exporter` resources
Download [cr.yaml](https://github.com/banzaicloud/prometheus-jmx-exporter-operator/blob/master/deploy/cr.yaml) and customize it for your needs.

```
spec:
  labelSelector:
    app: dummyapp
```

The label selector specifies what pods the operator to watch. The operator will investigate the pods that have labels which matches what is specified in `labelSelector`.
It checks the containers of the pod and the Java applications running in these containers.

It will load the Prometheus JMX Exporter into the Java applications and passes to it the configuration from the config map.

```
config:
    configMapName: prometheus-jmx-exporter-config
    configMapKey: config.yaml
```
Note: see [Prometheus JMX Exporter Configuration](#prometheus_jmx_exporter_configuration) above

```
port: 9400
```

This is the port number at which Prometheus server can scrape the metrics exported by Prometheus JMX exporter.
Chose a port that is not conflicting with any of the container ports already used by the applications running in the containers.

#### List the JMX Exporter endpoints managed by the operator
```
kubectl get prometheusjmxexporter
kubectl get prometheusjmxexporter <name-of-the-prometheus-jmx-exporter> -o yaml
```

The `status` section lists the endpoints.


## Limitations

Currently only one Java process per pod is being supported. In case a pod has multiple containers than the operator will select the first container from the list of containers
returned by Kubernetes API. Also if a container has multiple java processes running than the operator works on the first process.

## Future work

* Currently the user is required to provide a port for the Prometheus JMX Exporter that doesn't conflict with ports already being used
by processes running on the pod. The operator should take care of this by choosing a free port available in the container.

* Investigate the possibility of supporting pods with multiple containers and multiple Java processes per container.
This operator best works with microservices where there is one process per container.