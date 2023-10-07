目录
=================
  * [一、前言](#一前言)
  * [二、功能](#二功能)
  * [三、部署](#三部署)
  * [四、CLB功能](#六CLB功能)

# 一、前言
tencent cloud controller manager 主要是因为 tencent 官网的太老了，目前版本可以简易支持 k8s 1.26 的 service loadbanlancer 功能

# 二、功能
当前 tencentcloud-cloud-controller-manager 实现了:

    servicecontroller - 当集群中创建了类型为 LoadBalancer 的 service 的时候，创建相应的LoadBalancers。

# 三、部署

（1）创建secret
修改examples/secret.yaml:
```yaml

apiVersion: v1
kind: Secret
metadata:
  name: tencent-cloud-controller-manager-config
  namespace: kube-system
data:
  TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_REGION: "<REGION>"    #腾讯云区域
  TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_SECRET_ID: <SECRET_ID>"  #腾讯云帐号secret id
  TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_SECRET_KEY: "<SECRET_KEY>" #腾讯云帐号secret key
  TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_VPC_ID: "<VPC_ID>" #腾讯云的VPC ID
  TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_CLUSTER_ENDPOINT: "<ENDPOINT>" #腾讯云API的域名
```
将上面的value修改为你需要的配置，记得需要是base64编码.


创建：
```bash
kubectl apply -f ./examples/secret.yaml
```

（2）创建RBAC
```yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: cloud-controller-manager
  namespace: kube-system
 
---
apiVersion: v1
items:
  - apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      name: system:cloud-controller-manager
    rules:
      - apiGroups:
          - ""
        resources:
          - events
        verbs:
          - create
          - patch
          - update
      - apiGroups:
          - ""
        resources:
          - nodes
        verbs:
          - '*'
      - apiGroups:
          - ""
        resources:
          - nodes/status
        verbs:
          - patch
      - apiGroups:
          - ""
        resources:
          - services
        verbs:
          - list
          - patch
          - update
          - watch
      - apiGroups:
          - ""
        resources:
          - serviceaccounts
        verbs:
          - create
          - get
          - list
          - update
          - watch
      - apiGroups:
          - ""
        resources:
          - persistentvolumes
        verbs:
          - get
          - list
          - update
          - watch
      - apiGroups:
          - ""
        resources:
          - endpoints
        verbs:
          - create
          - get
          - list
          - watch
          - update
  - apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      name: system:cloud-node-controller
    rules:
      - apiGroups:
          - ""
        resources:
          - nodes
        verbs:
          - delete
          - get
          - patch
          - update
          - list
      - apiGroups:
          - ""
        resources:
          - nodes/status
        verbs:
          - patch
      - apiGroups:
          - ""
        resources:
          - events
        verbs:
          - create
          - patch
          - update
  - apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      name: system:pvl-controller
    rules:
      - apiGroups:
          - ""
        resources:
          - persistentvolumes
        verbs:
          - get
          - list
          - watch
      - apiGroups:
          - ""
        resources:
          - events
        verbs:
          - create
          - patch
          - update
kind: List
metadata: {}
 
---
apiVersion: v1
items:
  - apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRoleBinding
    metadata:
      name: system:cloud-node-controller
    roleRef:
      apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: system:cloud-node-controller
    subjects:
      - kind: ServiceAccount
        name: cloud-node-controller
        namespace: kube-system
  - apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRoleBinding
    metadata:
      name: system:pvl-controller
    roleRef:
      apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: system:pvl-controller
    subjects:
      - kind: ServiceAccount
        name: pvl-controller
        namespace: kube-system
  - apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRoleBinding
    metadata:
      name: system:cloud-controller-manager
    roleRef:
      apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: cluster-admin
    subjects:
      - kind: ServiceAccount
        name: cloud-controller-manager
        namespace: kube-system
kind: List
metadata: {}

```


```bash
kubectl apply -f ./examples/rbac.yaml
```

（3）创建Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: tencent-cloud-controller-manager
  name: tencent-cloud-controller-manager
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: tencent-cloud-controller-manager
  strategy:
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 1
    type: RollingUpdate
  template:
    metadata:
      labels:
        app: tencent-cloud-controller-manager
    spec:
      containers:
        - command:
            - /bin/tencent-cloud-controller-manager
            - --cloud-provider=tencentcloud # 指定 cloud provider 为 tencentcloud
            - --allocate-node-cidrs=true # 指定 cloud provider 为 tencentcloud 为 node 分配 cidr
            - --cluster-cidr=10.4.0.0/16 # 集群 pod 所在网络，需要提前创建
            - --cluster-name=global # 集群名称
            - --use-service-account-credentials
            - --configure-cloud-routes=true
            - --allow-untagged-cloud=true
            - --node-monitor-period=15s
            - --route-reconciliation-period=15s
            - -v=6
          env:
            - name: TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_REGION
              valueFrom:
                secretKeyRef:
                  key: TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_REGION
                  name: tencent-cloud-controller-manager-config
            - name: TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_SECRET_ID
              valueFrom:
                secretKeyRef:
                  key: TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_SECRET_ID
                  name: tencent-cloud-controller-manager-config
            - name: TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_SECRET_KEY
              valueFrom:
                secretKeyRef:
                  key: TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_SECRET_KEY
                  name: tencent-cloud-controller-manager-config
            - name: TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_VPC_ID
              valueFrom:
                secretKeyRef:
                  key: TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_VPC_ID
                  name: tencent-cloud-controller-manager-config
            - name: TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_CLUSTER_ENDPOINT
              valueFrom:
                secretKeyRef:
                  key: TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_CLUSTER_ENDPOINT
                  name: tencent-cloud-controller-manager-config
          image: yichanglu/tencent-cloud-controller-manager:v1.2.3
          imagePullPolicy: IfNotPresent
          name: tencent-cloud-controller-manager
          resources: {}
          terminationMessagePath: /dev/termination-log
          terminationMessagePolicy: File
      dnsPolicy: Default
      hostNetwork: true
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext: {}
      serviceAccountName: cloud-controller-manager
      terminationGracePeriodSeconds: 30
      tolerations:
        - effect: NoSchedule
          key: node.cloudprovider.kubernetes.io/uninitialized
          value: "true"
        - effect: NoSchedule
          key: node.kubernetes.io/network-unavailable
        - effect: NoSchedule
          key: kubernetes.io/role
          value: master

```

修改examples/deployment.yaml的启动参数：
```
            - --cluster-cidr=10.248.0.0/17 # 集群 pod 所在网络，需要提前创建
            - --cluster-name=kubernetes # 集群名称
```

```bash
kubectl apply -f ./examples/deployment.yaml
```

# 四、CLB功能
当用户创建类型是**LoadBalancer**的Service，默认情况下，tencent cloud controller manager会联动的创建CLB。而当用户删除此Service时，tencent cloud controller manager也会联动的删除CLB。  

下面是一个LoadBalancer类型的service例子
```yaml
apiVersion: v1
kind: Service
metadata:
  annotations:
    service.beta.kubernetes.io/tencentcloud-loadbalancer-type: private # 表示私网负载均衡器
    service.beta.kubernetes.io/tencentcloud-loadbalancer-type-internal-subnet-id: subnet-38h3a0fu # 表示提供负载均衡器IP的私网
    service.beta.kubernetes.io/tencentcloud-loadbalancer-node-label-key: kubernetes.io/hostname  # 表示负载均衡器的后端instance的标签key
    service.beta.kubernetes.io/tencentcloud-loadbalancer-node-label-value: 10.0.0.11  # 表示负载均衡器的后端instance的标签value
  labels:
    app: nginx
    service: nginx
  name: nginx
  namespace: default
spec:
  ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: 80
  selector:
    app: nginx
  sessionAffinity: None
  type: LoadBalancer
 
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
        - name: nginx-container
          image: nginx:latest
          ports:
            - containerPort: 80
```