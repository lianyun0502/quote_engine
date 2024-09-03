# Quote Engine

行情引擎，透過 websocket 串流行情資料，並透過 share memory 來傳遞行情資料。


## Index
- [Introduction](#Introduction)
- [Environment](#Environment)
- [Installation](#Installation)
- [Build](#Build)
- [Execute](#Execute)
- [Config 格式](#Config-格式)
- [Config 說明](#Config-說明)


## Environment
* Go version: go version go1.22.6 linux/amd64
* OS: Ubuntu Ubuntu 22.04.3 LTS

## Installation

there are two ways to install and use the package, one is to clone the repository and refer to local path and the other is to use the `go get` command set to `go.mod`.

### Git clone the repository

1. first clone the repository into your project directory

    ```bash
    git clone https://github.com/lianyun0502/quote_engine.git
    ```

    Your directory structure should look like this:

    ```bash
    your_project/
    ├── build/
    ├── quote_engine.go
    └── go.mod
    ```
    
2. replace the import refernce with the path of the repository in your project.

    ```bash
    go mod edit -replace=github.com/lianyun0502/quote_engine=../quote_engine
    ```

3. import the package in your project

    ```Go
    import (
        "github.com/lianyun0502/quote_engine"
    )
    ```

### Install the package use `go get`

1. use the `go get` command to install the package

    ```bash
    go get github.com/lianyun0502/quote_engine
    ```
2. import the package in your project

    ```Go
    import (
        "github.com/lianyun0502/quote_engine"
    )
    ```

## Example

```Go   
package main

import (
	"github.com/sirupsen/logrus"
	. "github.com/lianyun0502/quote_engine"
)

func main() {
    // load config file
	config, err := LoadConfig("config.yaml")
	if err != nil {
		logrus.Println(err)
		return
	} 

    // create quote engine
	quoteEngine := NewQuoteEngine(config)

    // start the quote engine
	quoteEngine.WsAgent.StartLoop()

    // wait for done signal
	<- quoteEngine.DoneSignal
}
```


## Build

1. entry the build directory

    ```bash
    cd build
    ```
2. build the project

    ```bash
    go build -o ./you_dir/quote_engine 
    ```

## Execute

After building the project, put config file `config.yaml` in directory and you can execute the project by the following command.
1. execute the project

    ```bash
    ./quote_engine
    ```


## Config 格式

```yaml
# logger 相關設定
Log:
  dir: "log/" 
  link_name: "latest_log.log"
  level: "debug"
  report_caller: false
  format: "2006-01-02 15:04:05.000000"
  writer:                                  # log file writer 相關設定 
    - name: "Info"
      path: "%Y%m%d_%H%M.log"
      max_age: 480
      rotation_time: 2
    - name: "Warn"
      path: "warn/%Y%m%d_%H%M.log"
      max_age: 480
      rotation_time: 2
  write_map:                               # log level 與 writer 的對應 
    panic: "Warn"
    fatal: "Warn"
    error: "Warn"
    warn: "Warn"
    info: "Info"
    debug: "Info"

# websocket client 相關設定    
Websocket:
  - exchange: "bybit"
    url: "wss://stream.bybit.com/v5/public/linear"
    subscribe: 
      # - "orderbook.50.BTCUSDT"
      # - "publicTrade.BTCUSDT"
      - "tickers.BTCUSDT"
    reconn_time: 10
    cmd:
      - method: "depth"
        params:
          symbol: "BTCUSDT"
          limit: 5
    publisher:
      - topic: "orderbook.50.BTCUSDT"
        skey: 177
        size: 1073741 
      - topic: "publicTrade.BTCUSDT"
        skey: 277
        size: 1073741 
      - topic: "tickers.BTCUSDT"
        skey: 377
        size: 1073741 
```

## Config 說明

### Log

logger 設定，可以設定多個log file writer，透過設定write_map來將log level對應到writer去分檔寫入不同 level 的 log。

| Key | Type | Description |
| --- | --- | --- |
| dir | string | log file directory |
| link_name | string | link name of the latest log file |
| level | string | log level ( debug, info, warn, error, fatal, panic ) |
| report_caller | bool | whether report the caller info |
| format | string | log time format |
| writer | list | list of log file writer |
| write_map | map | log level and writer mapping |

#### Log.writer 

log writter 設定，max_age 為 log file 最大存放時間 (單位: 小時)，rotation_time 為每個 log file 紀錄時間長度 (單位: 小時)。
| Key | Type | Description |
| --- | --- | --- |
| name | string | writer name |
| path | string | log file path |
| max_age | int | max age of the log files store in hours |
| rotation_time | int | data storage time in hours per file |


### Websocket

行情 streaming client 設定，可以設定多個交易所的多種行情資料串流。 透過設定subscribe來訂閱所需行情，透過設定Websocket.publisher去建立publisher來發布行情。 

| Key | Type | Description |
| --- | --- | --- |
| exchange | string | exchange name ( bybit, binance ...) |
| url | string | data stream url (參考交易所文件)|
| subscribe | list | list of topics to subscribe (參考交易所文件) |
| reconn_time | int | retry times of reconnection, set -1 will always retry |
| cmd | list | list of commands to send after connection |
| publisher | list | list of topics to publish |

#### Websocket.publisher

可以設定多個topic，每個topic會建立一個share memory buffer，用來傳遞data。

| Key | Type | Description |
| --- | --- | --- |
| topic | string | topic name (與訂閱行情相同, 會擷取topic字串，ex. orderbook.50.BTCUSDT -> orderbook )|
| skey | int | share memory 代號 |
| size | int | max size of the share memory buffer |


