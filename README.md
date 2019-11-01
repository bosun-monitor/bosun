# bosun

Bosun is a time series alerting framework developed by Stack Exchange. Scollector is a metric collection agent. Learn more at [bosun.org](http://bosun.org).

[![Build Status](https://travis-ci.org/bosun-monitor/bosun.svg?branch=master)](https://travis-ci.org/bosun-monitor/bosun/branches)

## 新增以下特性

1. 数值比较时，如果真则返回原值，而不是返回1。只对 >, <, >=, <= 有效

   - 例如 3.0 > 2，则返回 3.0。3.0 < 2，返回0
   - 注意 0 > -1 返回0（操作符前者），但实际是true，所以如果有可能是这种情况，请写作 -1 < 0
   - 返回原值的好处是，在告警模板里，可直接引用 `{{.Result.Value}}`得到告警的值，而不需要写`{{$v := .Eval .Alert.Vars.val }}`去获取再引用 $v

2. 在 notification 中加入`afterAction`选项，选项可取值`ForceClose` / `Forget` / `Purge`

   - 该选项一般用在 notification chain中，比如我定义了一个告警链 gcsx --> gcsx2  --> gcsx3，即gcsx通知直接发出,；等20分钟后如果告警还没恢复，发送gcsx2；再等6h如果高级还未恢复，则发送gcsx3。

     gcsx3 只有2种行为：一是发送完结束，那么在这条告警被 ack 之前，新的告警将永不会发出，这可能并不是我们想要的。二是gcsx3定义成循环告警，只要告警未被ack之前，告警一直发，哪怕指标已经恢复正常。

     在notification中定义 afterAction，就有了第3种行为：发送完通知后，立即自动 close/purge/forget 该条告警。如果告警还未恢复，则会产生一个新的告警，重复上面的过程；如果告警已恢复，流程结束，状态正常。

   - 一般不与 timeout, next一起使用

3. 在 alert 中加入 `DelayCloseNormal`选项，用于在告警指标恢复至normal后，自动close

   - 原来的流程里面，在告警发生之后，如果在发送gcsx2之前恢复正常，告警链会被正常终止，但因为这条告警如果没有手动ack，以后的新告警通知也不会发出。所以为了解决这个问题，引入`DelayCloseNormal=2m`这样的选项，表示在指标正常2分钟后，自动close该条告警，以免意外的丢失后面的告警，也可以解决告警flaping的问题。
   - 选项的另一个用途，是结合`unknownIsNormal=true`使用，在该指标丢失之后，忽略并close （正常会有专门检查心跳的指标用于告警）
   - `DelayCloseNormal`设置一般不少于`CheckFrequency`，因为只有每次check的时候才回去判断close normal。也一般设置小于notification chain之间的timeout时间，否则可能会发两条告警之后才close。

4. 增加每天定时屏蔽告警的功能

   - 比如每天凌晨1点-7点有备份，会引起大量磁盘和网络IO，在web页面设置 `start time period `,`end time period `开启定时屏蔽
   - 格式`01:00 +0800`, `07:00 +0800（+0800表示的是东8区时间），`这种情况一般把 duration  设置比较大的值，如`10000d` 。start值可以比end值大，表示的是屏蔽 start~24:00, 00~end



## building

To build bosun and scollector, clone to `$GOPATH/src/bosun.org`:

```
$ go get bosun.org/cmd/bosun
```

bosun and scollector are found under the `cmd` directory. Run `go build` in the corresponding directories to build each project.

## developing

Install:

* `npm install typescript@<version> -g` to be able to compile the ts files to js files. The current version of typescript to install will be in the `.tavis.yml` file in the root of this repo.
* `go get github.com/mjibson/esc` to embed the static files. Run `go generate` in `cmd/bosun` when new static assets (like JS and CSS files) are added or changed.

The `w.sh` script will automatically build and run bosun in a loop.
It will update itself when go/js/ts files change, and it runs in read-only mode, not sending any alerts.

```
$ cd cmd/bosun
$ ./w.sh
```

Go Version:
  * See the version number in `.travis.yml` in the root of this repo for the version of Go to use. Generally speaking, you should be able to use newer versions of Go if you are able to build Bosun without error.

Miniprofiler:
 * Bosun includes [miniprofiler](https://github.com/MiniProfiler/go) in the web UI which can help with debugging. The key combination `ALT-P` will show miniprofiler. This allows you to see timings, as well as the raw queries sent to TSDBs.
