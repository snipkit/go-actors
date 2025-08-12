protoc -I="../actor" -I="." \
  --go_out=. \
  --go-vtproto_out=. \
  --go_opt=paths=source_relative \
  cluster.proto
