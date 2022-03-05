# BBB

Simple tool to bombard russian sites.

_for educational purpose only :)_

### RUN

```shell
docker-compose -f docker-compose.yml up -d
```

### Run with VPN

```shell
VPNSP=*** OPENVPN_USER=*** OPENVPN_PASSWORD=*** docker-compose -f docker-compose-vpn.yml up -d
```

For vpn setup see: https://github.com/qdm12/gluetun

For better effect add cron job to restart containers from time to time:

```cron
*/5 * * * * /usr/local/bin/docker-compose restart bbb
```

This will change your IP address every 5 minutes.

## Why bbb

Most other simple-to-use tools do simple GET requests to the provided urls.
But such requests are easily handled by ddos protection sofware and bot detection scripts.
BBB can be launched in to modes:

1. `http` - does the same thing as other simple tools
2. `rod` (default) - runs the headless chrome browser with the help of [Rod](https://go-rod.github.io).
This helps to bypass some anti-ddos protections (with the help of some custom code), bypass bot detection scripts (with the help of [Rod-Stealth](https://github.com/go-rod/stealth)). Rod implementation runs slower, but can make more troubles.

## Features

* Runs out of box with zero configuration
* Custom config can be provided. Just mount volume to /opt/bbb/config with custom config.json
* Full support of golang text.template features for urls. See config/url_context.go
* Extendable (but a little bit dirty)) implementation. Fill free to pull request your variants

## Config

```javascript
{
  "Urls": [
    {
      "Url": "https://lenta.ru/news/{{.RandDate.Format \"2006/01/02\"}}/{{.RandStr}}/"
    },
    //...
  ],
  "Http": {
    "Header": {
      "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:97.0) Gecko/20100101 Firefox/97.0",
      //...
    }
  },
  "Rod": {
    "ShowUi": true|false //for debug
  },
  "Workers": {
    "Count": 500, //threads or browser tabs count
    "Timeout": 3
  },
  "Log": {
    "Level": "error|warning|default|verbose" //optional
  }
}
```

## TODO

* Support of https://github.com/opengs/uashield targets and proxies. Pull them automatically 