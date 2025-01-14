# 产品概览

该项目提供了一个 Operator，用于在 Kubernetes 上管理 EMQX 集群。

## 部署 EMQX Operator 

### 准备环境

EMQX Operator 部署前，请确认以下组件已经安装： 

|   软件                   |   版本要求       |
|:-----------------------:|:---------------:|
|  [Helm](https://helm.sh)                 |  >= 3           |
|  [cert-manager](https://cert-manager.io) |  >= 1.1.6       |

### 安装 EMQX Operator 

> 请先确认 [cert-manager](https://cert-manager.io) 已经就绪

```bash
helm repo add emqx https://repos.emqx.io/charts
helm repo update
helm install emqx-operator emqx/emqx-operator --namespace emqx-operator-system --create-namespace
```

检查 EMQX Operator 是否就绪

```bash
kubectl get pods -l "control-plane=controller-manager" -n emqx-operator-system
```

输出类似于：

```bash
NAME                                                READY   STATUS    RESTARTS   AGE
emqx-operator-controller-manager-68b866c8bf-kd4g6   1/1     Running   0          15s
```

### 升级 EMQX Operator 

执行下面的命令可以升级 EMQX Operator，若想指定到升级版只需要增加 --version=x.x.x 参数即可

```bash 
helm upgrade emqx-operator emqx/emqx-operator -n emqx-operator-system 
```

> 不支持 1.x.x 版本 EMQX Operator 升级到 2.x.x 版本。

### 卸载 EMQX Operator 

执行如下命令卸载 EMQX Operator

```bash
helm uninstall emqx-operator -n emqx-operator-system
```

## 部署 EMQX

### 部署 EMQX 5

1. 部署 EMQX 

```bash
cat << "EOF" | kubectl apply -f -
apiVersion: apps.emqx.io/v2alpha1
kind: EMQX
metadata:
  name: emqx
spec:
  image: emqx/emqx:5.0.14
EOF
```

2. 检查 EMQX 集群是否就绪

```bash
kubectl get emqx emqx -o json | jq '.status.conditions[] | select( .type == "Running" and .status == "True")'
```

这可能需要等待一段时间命令才会执行成功，因为需要等待所有的 EMQX 节点启动并加入集群。

输出类似于：

```bash 
{
  "lastTransitionTime": "2023-02-10T02:46:36Z",
  "lastUpdateTime": "2023-02-07T06:46:36Z",
  "message": "Cluster is running",
  "reason": "ClusterRunning",
  "status": "True",
  "type": "Running"
}
```

### 部署 EMQX 4

1. 部署 EMQX 

```bash
cat << "EOF" | kubectl apply -f -
apiVersion: apps.emqx.io/v1beta4
kind: EmqxBroker
metadata:
  name: emqx
spec:
  template:
    spec:
      emqxContainer:
        image:
          repository: emqx/emqx-ee
          version: 4.4.14
EOF
```

2. 检查 EMQX 集群是否就绪

```bash
kubectl get emqxBroker emqx -o json | jq '.status.conditions[] | select( .type == "Running" and .status == "True")'
```

这可能需要等待一段时间命令才会执行成功，因为需要等待所有的 EMQX 节点启动并加入集群。

输出类似于：

```bash 
{
  "lastTransitionTime": "2023-02-13T02:38:25Z",
  "lastUpdateTime": "2023-02-13T02:44:19Z",
  "message": "All resources are ready",
  "reason": "ClusterReady",
  "status": "True",
  "type": "Running"
}
```

### 部署 EMQX Enterprise 4

1. 部署 EMQX 

```bash
cat << "EOF" | kubectl apply -f -
apiVersion: apps.emqx.io/v1beta4
kind: EmqxEnterprise
metadata:
  name: emqx-ee
spec:
  template:
    spec:
      emqxContainer:
        image:
          repository: emqx/emqx-ee
          version: 4.4.14
EOF
```

2. 检查 EMQX 集群是否就绪

```bash 
kubectl get emqxEnterprise emqx-ee -o json | jq '.status.conditions[] | select( .type == "Running" and .status == "True")'
```

这可能需要等待一段时间命令才会执行成功，因为需要等待所有的 EMQX 节点启动并加入集群。

输出类似于：

```bash 
{
  "lastTransitionTime": "2023-02-13T02:38:25Z",
  "lastUpdateTime": "2023-02-13T02:44:19Z",
  "message": "All resources are ready",
  "reason": "ClusterReady",
  "status": "True",
  "type": "Running"
}
```
