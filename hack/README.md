## Bringup KIND Cluster with multiple nodes
```
kind create cluster --config kind-config.yaml
```
## set namespace for the controller
```
export WATCH_NAMESPACE=metallb-system
```
## Apply address-pool sample config
```
kubectl apply -f config/samples/metallb.addresspool.yaml
kubectl get addresspool -n metallb-system addresspool-sample -o yaml
kubectl get configmap -n metallb-system config -o yaml
```

## Use NGINX service to test loadbalancer service
```
kubectl create deploy nginx --image nginx

kubectl expose deploy nginx --port 80 --type LoadBalancer

kubectl get services

You'll get out put similar to the following:

NAME         TYPE           CLUSTER-IP      EXTERNAL-IP    PORT(S)        AGE
kubernetes   ClusterIP      10.96.0.1       <none>         443/TCP        11m
nginx        LoadBalancer   10.96.148.110   172.19.255.1   80:32326/TCP   9s
Notice that the nginx service publishes an EXTERNAL-IP.
 Run the curl against the EXTERNAL-IP, for example:

curl 172.19.255.1

You'll get output similar to the following:

<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
    body {
        width: 35em;
        margin: 0 auto;
        font-family: Tahoma, Verdana, Arial, sans-serif;
    }
</style>
</head>
<body>
<h1>Welcome to nginx!</h1>
<p>If you see this page, the nginx web server is successfully installed and
working. Further configuration is required.</p>

<p>For online documentation and support please refer to
<a href="http://nginx.org/">nginx.org</a>.<br/>
Commercial support is available at
<a href="http://nginx.com/">nginx.com</a>.</p>

<p><em>Thank you for using nginx.</em></p>
</body>
</html>
```