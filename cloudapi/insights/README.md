To generate proto files, please run the following command from the root of the project:

```bash
protoc --proto_path=cloudapi/insights/proto \
--go_out=cloudapi/insights/proto --go_opt=paths=source_relative \
--go-grpc_out=cloudapi/insights/proto --go-grpc_opt=paths=source_relative \
$(find cloudapi/insights/proto -iname "*.proto")
```
