curl -XPOST http://127.0.0.1:9667/register -d '    {
        "config": {
            "host": "localhost",
            "port": 6667,
            "nickname": "bot",
            "realname": "IRC Bot",
            "appname": "application",
            "messageurl": "http://localhost:9999/"
        }
    }
'
