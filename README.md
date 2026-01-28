# EdgeX 应用服务开发demo

## 1.命名规则

`
app-[服务名]-go
`

## 2.开发指导
### 2.1 命名替换
- 将所有的`app-demo-go`替换为`app-[服务名]-go`
- 将所有的`demo`替换为`[服务名]`

### 2.3 启动核心服务环境
```bash
cd cmd/docker-compose

docker compose -f docker-compose-core.yml up -d
```

### 2.4 本地运行测试
```bash
make clean

make

cd cmd/

./app-[服务名]-go -o -d -cp
```

### 2.5 docker启动测试
```bash
make clean

make build-[amd64 or arm64]

make docker-[amd64 or arm64]

cd cmd/docker-compose

docker compose -f docker-compose-demo.yml up
```
![](docs/image.png)