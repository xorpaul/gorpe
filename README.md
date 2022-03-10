
Link to check_gorpe: https://github.com/xorpaul/check_gorpe

# GORPE

```
./gorpe -fg -debug
```
gorpe.gcfg
```
[commands]
sleep1=sleep 1
# with args
sleep_arg=sleep "$ARG$"
echo_args=echo "$ARG$ and $ARG$"
```

```
$ curl -k https://127.0.0.1:5666/sleep1
Received no text
Returncode: 0
$ curl -k https://127.0.0.1:5666/sleep_arg/5
Received no text
Returncode: 0
$ curl -k https://127.0.0.1:5666/echo_args/foo/bar
foo and bar
Returncode: 0
```

```
2015/06/05 16:30:22 DEBUG xPLDnJOb [Incoming GET request from IP: 127.0.0.1]
2015/06/05 16:30:22 DEBUG xPLDnJOb [Request path:  /sleep1]
2015/06/05 16:30:22 DEBUG xPLDnJOb [Request path parts are: %q [sleep1]]
2015/06/05 16:30:22 DEBUG xPLDnJOb [Found command:  sleep1]
2015/06/05 16:30:22 DEBUG xPLDnJOb [Found command arguments: %q []]
2015/06/05 16:30:22 DEBUG xPLDnJOb [Found %q command arguments in this command 0]
2015/06/05 16:30:22 DEBUG xPLDnJOb [Got command from config:  sleep 1]
2015/06/05 16:30:22 DEBUG xPLDnJOb [Replacing arguments and executing:  sleep 1]
2015/06/05 16:30:22 DEBUG xPLDnJOb [checkArguments are %q:  [1]]
2015/06/05 16:30:22 DEBUG xPLDnJOb [checkScript:  sleep]
2015/06/05 16:30:22 DEBUG xPLDnJOb [checkArguments are %q:  [1]]
2015/06/05 16:30:23 DEBUG xPLDnJOb [out:  ]
2015/06/05 16:30:23 DEBUG xPLDnJOb Got output: []
2015/06/05 16:30:23 DEBUG xPLDnJOb Got return code: [0]
2015/06/05 16:30:32 DEBUG CsNVlgTe [Incoming GET request from IP: 127.0.0.1]
2015/06/05 16:30:32 DEBUG CsNVlgTe [Request path:  /sleep_arg/5]
2015/06/05 16:30:32 DEBUG CsNVlgTe [Request path parts are: %q [sleep_arg 5]]
2015/06/05 16:30:32 DEBUG CsNVlgTe [Found command:  sleep_arg]
2015/06/05 16:30:32 DEBUG CsNVlgTe [Found command arguments: %q [5]]
2015/06/05 16:30:32 DEBUG CsNVlgTe [Found %q command arguments in this command 1]
2015/06/05 16:30:32 DEBUG CsNVlgTe [Got command from config:  sleep $ARG$]
2015/06/05 16:30:32 DEBUG CsNVlgTe [Replacing $ARG$ with %q results in %q 5 sleep 5]
2015/06/05 16:30:32 DEBUG CsNVlgTe [Replacing arguments and executing:  sleep 5]
2015/06/05 16:30:32 DEBUG CsNVlgTe [checkArguments are %q:  [5]]
2015/06/05 16:30:32 DEBUG CsNVlgTe [checkScript:  sleep]
2015/06/05 16:30:32 DEBUG CsNVlgTe [checkArguments are %q:  [5]]
2015/06/05 16:30:37 DEBUG CsNVlgTe [out:  ]
2015/06/05 16:30:37 DEBUG CsNVlgTe Got output: []
2015/06/05 16:30:37 DEBUG CsNVlgTe Got return code: [0]
2015/06/05 16:30:40 DEBUG MaPEZQle [Incoming GET request from IP: 127.0.0.1]
2015/06/05 16:30:40 DEBUG MaPEZQle [Request path:  /echo_args/foo/bar]
2015/06/05 16:30:40 DEBUG MaPEZQle [Request path parts are: %q [echo_args foo bar]]
2015/06/05 16:30:40 DEBUG MaPEZQle [Found command:  echo_args]
2015/06/05 16:30:40 DEBUG MaPEZQle [Found command arguments: %q [foo bar]]
2015/06/05 16:30:40 DEBUG MaPEZQle [Found %q command arguments in this command 2]
2015/06/05 16:30:40 DEBUG MaPEZQle [Got command from config:  echo $ARG$ and $ARG$]
2015/06/05 16:30:40 DEBUG MaPEZQle [Replacing $ARG$ with %q results in %q foo echo foo and $ARG$]
2015/06/05 16:30:40 DEBUG MaPEZQle [Replacing $ARG$ with %q results in %q bar echo foo and bar]
2015/06/05 16:30:40 DEBUG MaPEZQle [Replacing arguments and executing:  echo foo and bar]
2015/06/05 16:30:40 DEBUG MaPEZQle [checkArguments are %q:  [foo and bar]]
2015/06/05 16:30:40 DEBUG MaPEZQle [checkScript:  echo]
2015/06/05 16:30:40 DEBUG MaPEZQle [checkArguments are %q:  [foo and bar]]
2015/06/05 16:30:40 DEBUG MaPEZQle [out:  foo and bar
]
2015/06/05 16:30:40 DEBUG MaPEZQle Got output: [foo and bar
]
2015/06/05 16:30:40 DEBUG MaPEZQle Got return code: [0]
```

# How to execute commands on remote systems via GORPE
The URI of the POST request gets used as the new command handler:

```
$ curl -k https://127.0.0.1:5666/exit1
exiting with 1
Returncode: 1
```
$ curl -k https://127.0.0.1:5666/exit0
exiting with 0
Returncode: 0

```
2015/06/05 16:54:17 DEBUG FpLSjFbc [Incoming GET request from IP: 127.0.0.1]
2015/06/05 16:54:17 DEBUG FpLSjFbc [Request path:  /exit1]
2015/06/05 16:54:17 DEBUG FpLSjFbc [Request path parts are: %q [exit1]]
2015/06/05 16:54:17 DEBUG FpLSjFbc [Found command:  exit1]
2015/06/05 16:54:17 DEBUG FpLSjFbc [Found command arguments: %q []]
2015/06/05 16:54:17 DEBUG FpLSjFbc [Found %q command arguments in this command 0]
2015/06/05 16:54:17 DEBUG FpLSjFbc [Got command from config:  /tmp/exit1]
2015/06/05 16:54:17 DEBUG FpLSjFbc [Replacing arguments and executing:  /tmp/exit1]
2015/06/05 16:54:17 DEBUG FpLSjFbc [checkScript:  /tmp/exit1]
2015/06/05 16:54:17 DEBUG FpLSjFbc [checkArguments are %q:  []]
2015/06/05 16:54:17 DEBUG FpLSjFbc [out:  exiting with 1
]
2015/06/05 16:54:17 DEBUG FpLSjFbc Got output: [exiting with 1
]
2015/06/05 16:54:17 DEBUG FpLSjFbc Got return code: [1]
2015/06/05 16:54:18 DEBUG XoEFfRsW [Incoming GET request from IP: 127.0.0.1]
2015/06/05 16:54:18 DEBUG XoEFfRsW [Request path:  /exit2]
2015/06/05 16:54:18 DEBUG XoEFfRsW [Request path parts are: %q [exit2]]
2015/06/05 16:54:18 DEBUG XoEFfRsW [Found command:  exit2]
2015/06/05 16:54:18 DEBUG XoEFfRsW [Found command arguments: %q []]
2015/06/05 16:54:18 DEBUG XoEFfRsW [Found %q command arguments in this command 0]
2015/06/05 16:54:18 DEBUG XoEFfRsW [Got command from config:  /tmp/exit2]
2015/06/05 16:54:18 DEBUG XoEFfRsW [Replacing arguments and executing:  /tmp/exit2]
2015/06/05 16:54:18 DEBUG XoEFfRsW [checkScript:  /tmp/exit2]
2015/06/05 16:54:18 DEBUG XoEFfRsW [checkArguments are %q:  []]
2015/06/05 16:54:18 DEBUG XoEFfRsW [out:  exiting with 2
]
2015/06/05 16:54:18 DEBUG XoEFfRsW Got output: [exiting with 2
]
2015/06/05 16:54:18 DEBUG XoEFfRsW Got return code: [2]
2015/06/05 16:54:19 DEBUG xPLDnJOb [Incoming GET request from IP: 127.0.0.1]
2015/06/05 16:54:19 DEBUG xPLDnJOb [Request path:  /exit3]
2015/06/05 16:54:19 DEBUG xPLDnJOb [Request path parts are: %q [exit3]]
2015/06/05 16:54:19 DEBUG xPLDnJOb [Found command:  exit3]
2015/06/05 16:54:19 DEBUG xPLDnJOb [Found command arguments: %q []]
2015/06/05 16:54:19 DEBUG xPLDnJOb [Found %q command arguments in this command 0]
2015/06/05 16:54:19 DEBUG xPLDnJOb [Got command from config:  /tmp/exit3]
2015/06/05 16:54:19 DEBUG xPLDnJOb [Replacing arguments and executing:  /tmp/exit3]
2015/06/05 16:54:19 DEBUG xPLDnJOb [checkScript:  /tmp/exit3]
2015/06/05 16:54:19 DEBUG xPLDnJOb [checkArguments are %q:  []]
2015/06/05 16:54:19 DEBUG xPLDnJOb [out:  exiting with 3
]
2015/06/05 16:54:19 DEBUG xPLDnJOb Got output: [exiting with 3
]
2015/06/05 16:54:19 DEBUG xPLDnJOb Got return code: [3]
```

