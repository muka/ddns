# Dynamic DNS Service

## Example Usage

Run `go run main.go --tsig some_key:c29tZV9rZXk=`

### With nsupdate

Update with `nsupdate nsupdate.txt`

Test `nslookup test1.local.lan localhost -port=10053`

### Rest API

Using gRPC (`:50551`) / HTTP (`:5551`) endpoint

```
curl -X POST http://localhost:5551/v1/record \
  -d '{
	"domain": "foobar800.local.lan",
	"type": "A",
	"expires": 123456789
}'
```

Test `nslookup test1.local.lan localhost -port=10053`

## License

MIT License
