napp-operator
=============

Kubernetes Operator for managing 2 Nginx apps, first depending on the second.


Initial project generation steps
================================

* Initialize Go module
```
go mod init example.com/napp-operator
```

* Create new project with `kubebuilder`
```
kubebuilder init --domain example.com
```

* Create new api for InsurancePlatform kind
```
kubebuilder create api --group apps --version v1 --kind NginxApp
```


Running operator in development mode
====================================

Please make sure that `kubectl` is installed and configured to the right Kubernetes cluster.

* Install operator CRDs on cluster
```
make install
```

* Run operator from console
```
make run
```

Removing the operator CRDs from the cluster:
```
make uninstall
```
Warning: this will also delete all resources deployed using the operator!


Testing the operator
====================

Packaging the operator into the Docker image and full deployment on Kubernetes cluster is beyond the scope
of this project, but the process is fairly straightforward and [well documented](https://kubebuilder.io/cronjob-tutorial/running.html).

The easiest way to test the operator is to run it in the development mode (see above) and then simply create NginxApp resource:
```
kubectl create -f config/samples/apps_v1_nginxapp.yaml -n some-namespace
```
