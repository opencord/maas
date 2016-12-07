# Config Gen
-----------
This service generates network configuration (`network-cfg.json`) for CORD POD

### Prerequisites:
- Install Go (only for local debugging)
- Make sure ONOS is reachable either locally or via SSH tunnel (either way is fine)
- Make sure devices, hosts are connected and showing up in ONOS
- Config-gen server listens on port 1337, so make sure it's available, if not change the ENV VAR `CONFIGGEN_ConfigServerPort` to the port you wish config-gen to use


### To run locally from source (without a container) for debugging:
- Run `go run congifGen.go`
- Use CURL to get the JSON output for `network-cfg.json` (CURL example below)


### To run the container/uService:
- Use the `Makefile` to build the container image with command `make image`
- Run the container/service with `make run`
- Example client would use the following CURL command to get the config:
```bash
curl -H "Content-Type: application/json" -d '{"switchcount" : 4, "hostcount": 4, "onosip" : "127.0.0.1" }' http://localhost:1337/config/
```
- Where, `switchcount`, and `hostcount` are the number of switches & hosts expected to be in the network respectively, and onosip is the ONOS IP (default is 127.0.0.1)


### Note:
- Every parameter is set by envconfig (https://github.com/kelseyhightower/envconfig)
- For the parameters passed to the server through HTTP POST override the config set by envconfig
- To modify any of the envvars, please set the ENV VAR with the prefix of `CONFIGGEN`
- List of parameters set by envconfig:
```bash
	Port             string `default:"8181"`		//ONOS Port
	IP               string `default:"127.0.0.1"`	//ONOS IP (Default)
	SwitchCount      int    `default:"0"`
	HostCount        int    `default:"0"`
	Username         string `default:"karaf"`
	Password         string `default:"karaf"`
	LogLevel         string `default:"warning" envconfig:"LOG_LEVEL"`
	LogFormat        string `default:"text" envconfig:"LOG_FORMAT"`
	ConfigServerPort string `default:"1337"`		//Config-gen service port default
	ConfigServerIP   string `default:"127.0.0.1"`	//Config-gen service IP
```
