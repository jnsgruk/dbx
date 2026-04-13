sudo snap install k8s --classic --channel 1.34-classic/stable
sudo snap refresh --hold k8s

cat <<EOF | sudo k8s bootstrap --file -
containerd-base-dir: /opt/ck8s
cluster-config:
  # gateway:
  #   enabled: true
  network:
    enabled: true
  dns:
    enabled: true
  local-storage:
    enabled: true
  load-balancer:
    enabled: true
    l2-mode: true
    cidrs:
      - 127.0.0.1/32
EOF

cat <<EOF | sudo k8s kubectl apply -f-
apiVersion: v1
kind: Namespace
metadata:
  name: registry
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: registry
  namespace: registry
spec:
  selector:
    matchLabels:
      app: registry
  template:
    metadata:
      labels:
        app: registry
    spec:
      containers:
        - name: registry
          image: registry:2
          ports:
            - containerPort: 5000
          volumeMounts:
            - name: data
              mountPath: /var/lib/registry
      volumes:
        - name: data
          emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: registry
  namespace: registry
spec:
  type: NodePort
  selector:
    app: registry
  ports:
    - port: 5000
      targetPort: 5000
      nodePort: 30500
EOF

mkdir -p ~/.kube
sudo k8s config > ~/.kube/config