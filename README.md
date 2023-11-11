## go-micro
参考GeeRpc实现的微服务中客户端与服务端基于HTTP/TCP协议、基于JSON序列化方式的远程服务调用的DEMO。

rpc（C/S，HTTP是B/S）: 远程服务调用方法，其本身不只有通信协议（rpc: 可以是http、tcp、udp），还包括服务发现（http基于dns，ex: coreDns, rpc基于注册中心，ex: etcd、redis等），序列化方式（http: json、protobuf，rpc: protobuf、thrift）以及异常处理。