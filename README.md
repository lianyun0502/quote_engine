# Quote Engine

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
- [GRPC API](#GRPC-API)


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
  ├── main.go
  ├── config.yaml
  └── go.mod
  ```
2. install the package from your `go.mod` file

  ```bash
    go mod tidy
  ```
  

## Example

```Go   
package main

import (
  "github.com/sirupsen/logrus"
  . "github.com/lianyun0502/quote_engine/engine"
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
  go build -o ./you_dir/excutable_file_name 
  ```

## Execute

After building the project, put config file `config.yaml` in directory and you can execute the project by the following command.
1. execute the project

  ```bash
  ./excutable_file_name
  ```


## Config 格式

```yaml
# logger 相關設定
Log:
  dir: "log/"                                 # log file directory
  link_name: "latest_log.log"                
  level: "debug"                              # log level ( debug, info, warn, error, fatal, panic )
  report_caller: false                        # whether report the caller info           
  format: "2006-01-02 15:04:05.000000"
  writer:
  - name: "Info"
    path: "%Y%m%d_%H%M.log"
    max_age: 480
    rotation_time: 2
  - name: "Warn"
    path: "warn/%Y%m%d_%H%M.log"
    max_age: 480
    rotation_time: 2
  write_map:
  panic: "Warn"
  fatal: "Warn"
  error: "Warn"
  warn: "Warn"
  info: "Info"
  debug: "Info"

Data:                                         # 資料存放設定
  save: true                                  # 是否儲存資料
  dir: "data/"                                # 資料存放root     
  max_age: 480                                # 資料存放最大時間 (單位: 小時)
  rotation_time: 2                            # 每個資料檔案存放時間 (單位: 小時)      

GRPCServer:                                   # grpc server 相關設定
  port: "1687"                                # grpc server port
  host: "localhost"                           # grpc server host

  
Websocket:                                    # websocket client 相關設定
  - exchange: "binance"                       # 交易所名稱          
  url: "wss://stream.bybit.com/v5/public/spot" # 已棄用
  host_type: "spot"                         # 交易商品類型 (spot, future)
  ws_pool_size: 1                           # websocket pool size，開啟WS連線數量
  subscribe: 
  reconn_time: -1                           # 重連次數，-1 代表無限重連
  cmd:                                      # 已棄用
    - method: "depth"
    params:
      symbol: "BTCUSDT"
      limit: 5
  publisher:                                # 設定要發布的topic，及對應的share memory skey設定
    - topic: "orderbook"                    # orderbook topic
    skey: 158
    size: 1073741 #1GB
    store: true
    - topic: "Trade"                        # trade topic 
    skey: 258
    size: 1073741 #1GB
    store: true
    - topic: "ticker"                       # market data ticker topic
    skey: 358
    size: 1073741 #1GB
    store: true
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

### Data

資料儲存設定，決定是否儲存接收的行情資料及存放位置。

| Key | Type | Description |
| --- | --- | --- |
| save | bool | whether to save the data |
| dir | string | root directory of data storage |
| max_age | int | max age of the data files store in hours |
| rotation_time | int | data storage time in hours per file |

### GRPCServer

gRPC 服務設定，用於提供API服務。

| Key | Type | Description |
| --- | --- | --- |
| port | string | gRPC server port |
| host | string | gRPC server host |

### Websocket

行情 streaming client 設定，可以設定多個交易所的多種行情資料串流。 透過設定subscribe來訂閱所需行情，透過設定Websocket.publisher去建立publisher來發布行情。 

| Key | Type | Description |
| --- | --- | --- |
| exchange | string | exchange name ( bybit, binance ...) |
| host_type | string | 交易商品類型 (spot, future) |
| ws_pool_size | int | websocket pool size，開啟WS連線數量 |
| url | string | data stream url (參考交易所文件) [已棄用] |
| subscribe | list | list of topics to subscribe (參考交易所文件) |
| reconn_time | int | retry times of reconnection, set -1 will always retry |
| cmd | list | list of commands to send after connection [已棄用] |
| publisher | list | list of topics to publish |

#### Websocket.publisher

可以設定多個topic，每個topic會建立一個share memory buffer，用來傳遞data。

| Key | Type | Description |
| --- | --- | --- |
| topic | string | topic name (與訂閱行情相同, 會擷取topic字串，ex. orderbook.50.BTCUSDT -> orderbook )|
| skey | int | share memory 代號 |
| size | int | max size of the share memory buffer |
| store | bool | whether to store the data |


## GRPC API
| Method | Description |
| --- | --- |
| GetStatus | 取得引擎狀態，包括各交易所連線狀態 |
| GetOrderbook | 取得特定交易所和交易對的訂單簿資料 |
| GetTicker | 取得特定交易所和交易對的Ticker資料 |
| GetTrade | 取得特定交易所和交易對的最新成交資料 |
| Subscribe | 訂閱特定交易所和交易對的行情資料 |
| Unsubscribe | 取消訂閱特定交易所和交易對的行情資料 |