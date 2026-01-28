docker run -d \
  --name app-demo-go \
  --hostname edgex-app-demo-go \
  --network edgex_edgex-network \
  --read-only \
  --restart always \
  --security-opt no-new-privileges:true \
  --user 2002:2001 \
  -e EDGEX_SECURITY_SECRET_STORE="false" \
  -e SERVICE_HOST=app-demo-go \
  -v /etc/localtime:/etc/localtime:ro \
  linux/amd64/app-demo-go:0.0.0-dev \
  /app-demo-go \
  -cp=keeper.http://edgex-core-keeper:59890 \
  --registry