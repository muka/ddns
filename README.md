# Dynamic DNS Service

## Example Usage

### Rest API support

gRPC (`:50551`) / HTTP (`:5551`) endpoint are available

#### Set record

```bash
curl -X POST http://localhost:5551/v1/record \
  -H 'content-type: application/json' \
    -d '{
	"ip": "127.0.0.1",
	"domain": "foobar.local.lan",
	"type": "A",
	"expires": 123456789
}'
```

#### Remove Record

`curl -X DELETE http://localhost:5551/v1/record/foobar.local.lan/A`

#### Test Record

`nslookup foobar.local.lan localhost -port=10053`

### TSIG support

Run `go run main.go --tsig some_key:c29tZV9rZXk=`

#### Using nsupdate

Update with `nsupdate nsupdate.txt`

#### Test records

`nslookup test1.local.lan localhost -port=10053`

## License

MIT License
