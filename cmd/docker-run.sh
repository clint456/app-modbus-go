docker run -d \
  --name app-modbus-go \
  --hostname edgex-app-modbus-go \
  --network edgex_edgex-network \
  --read-only \
  --restart always \
  --security-opt no-new-privileges:true \
  --user 2002:2001 \
  -e EDGEX_SECURITY_SECRET_STORE="false" \
  -e SERVICE_HOST=app-modbus-go \
  -v /etc/localtime:/etc/localtime:ro \
  linux/amd64/app-modbus-go:0.0.0-dev \
  /app-modbus-go \
  -cp=keeper.http://edgex-core-keeper:59890 \
  --registry \
  - -cd=/res  \
  - -cf=configuration.yaml \
  - -o