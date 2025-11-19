# 延迟组件

## 创建延迟任务

请求参数
| 参数 | 说明 | 示例 |
|------------|------------|-----------|
| schema | 回调协议 | HTTP,GRPC,FMT |
| url | 回调URL | 回调URL |
| path | 回调路径 | PATH路径 |
| data | 回调数据 | JSON格式 |
| delay_time | 延迟时间,单位秒 | 20 |
| timeout | 超时时间,单位秒 | 3 |
| backoff | 重试时间间隔,单位秒 | [5,10,60] |

GRPC

```
# delay/api/delay
URL: 127.0.0.1:50051
PATH: delay.Delay/Register
{
    "schema": "HTTP",
    "url": "http://192.168.6.93:30081",
    "path": "/example/valid",
    "date": {
        "fields": {
            "result": {
                "stringValue": "SUCCESS"
            }
        }
    },
    "delay_time": 20,
    "timeout": 3,
    "backoff": [
        5,
        10,
        60
    ]
}
```

HTTP

```
curl --location --request POST 'http://127.0.0.1:8081/delay/register' \
--header 'Content-Type: application/json' \
--data-raw '{
    "schema": "HTTP", 
    "url": "http://192.168.6.93:30081",
    "path": "/example/valid",
    "data": {
        "result": "SUCCESS"
    },
    "delay_time": 20,
    "timeout": 3,
    "backoff": [
        5,
        10,
        60
    ]
}'
```

### 回调处理

回调方法BODY中返回`SUCCESS`为成功，其他为失败