```shell
GOOS=linux GOARCH=arm64 GOARM=7 go build -o turn-server-socket-arm64
```

## Windows 交叉编译
 
### 编译 linux amd64
```
set CGO_ENABLED=0
set GOOS=linux
set GOARCH=amd64
go build -o turn-server-socket-amd64
```

## Linux 交叉编译


## Docker镜像构建


### 从源码构建多架构
```shell
docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
docker buildx create --name webrtc-turn-server-builder --driver docker-container --platform linux/amd64,linux/arm64 --use
docker buildx inspect --bootstrap
docker buildx ls
docker buildx build --push --platform linux/amd64,linux/arm64 -t webrtc-turn-server -f Dockerfile .
```


### Docker 从运行包构建
```shell
docker build -t turn-server-socket . -f Dockerfile-Raw
```