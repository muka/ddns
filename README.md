# Dynamic DNS Service

Simple dynamic DNS service for LAN use

## Supported record

- A / AAAA + (PTR)
- CNAME
- MX

## Running

```bash
make build
./build/ddns --debug
```

or `go run cli/cli --debug`

## Rest API

Offers a gRPC (`:50551`) and HTTP/JSON (`:5551`) endpoint. See also generated [./api/api.swagger.json](./api/api.swagger.json) for usage reference.

### Create a record

```bash
curl -X POST http://localhost:5551/v1/record \
  -H 'content-type: application/json' \
    -d '{
	"ip": "127.0.0.1",
	"domain": "foobar.local.lan",
	"type": "A",
	"expires": 1498454965
}'
```

### Remove Record

`curl -X DELETE http://localhost:5551/v1/record/foobar.local.lan/A`

### Test Record

`nslookup foobar.local.lan localhost -port=10053`

## nsupdate support

Run `go run main.go --tsig some_key:c29tZV9rZXk=`

### Using nsupdate

Update with `nsupdate nsupdate.txt`

### Test records

`nslookup test1.local.lan localhost -port=10053`

## Credits

Inspired by [the post](http://mkaczanowski.com/golang-build-dynamic-dns-service-go/) of Mateusz Kaczanowski

## License

MIT License
