napp-operator
=============

Operator for managing 2 Nginx apps, first depending on the second.


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

