# Dynamic DNS Service

## Usage

- Run `go run main.go --port 10053 --tsig some_key:c29tZV9rZXk=`
- Update `nsupdate nsupdate.txt`
- Test `nslookup test1.local.lan localhost -port=10053`

## License

MIT License
