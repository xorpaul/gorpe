```
./gorpe -fg -debug
```

```
curl -k https://127.0.0.1:5666/sleep1
curl -k https://127.0.0.1:5666/sleep_arg/5
curl -k https://127.0.0.1:5666/echo_args/foo/bar
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
