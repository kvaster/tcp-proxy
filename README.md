# tcp-proxy

A simple TCP proxy server written in golang.

It acts as transparent stream proxy. You may use nftables to redirect tcp streams:

```
ip saddr <some addr> tcp dport <proxy port> dnat ip to <proxy addr>:<proxy port>
```
