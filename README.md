```
./gorpe -fg -debug
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

### Upload feature
A simple POST request saves the contents of the attached file in the directory configured with `upload_dir`:
```
$ curl -k -XPOST --header "Content-Type:multipart/form-data" -F file=@upload/exit0 https://127.0.0.1:5666/exit0
File exit0 uploaded successfully, sha256sum: 3e9ac084c7a364d5daffe5cce11bc9e0a390aa2ee8cb2b1b25e26007b65be37e
Result Code: 0
```
```
curl -k -XPOST --header "Content-Type:multipart/form-data" -F file=@upload/exit1 https://127.0.0.1:5666/exit1
File exit1 uploaded successfully, sha256sum: 67e1a85dd8179c51bc7a230ea849892079c82e35dbe6af46f99d508c3f192008
Result Code: 0
```

After the successfull upload a SAH256 checksum is printed as the HTTP response and in the debug log:

```
2015/06/05 16:53:57 DEBUG XVlBzgba [Incoming POST request from IP: 127.0.0.1]
2015/06/05 16:53:57 DEBUG XVlBzgba [Check Command: exit1  File  exit1 successfully uploaded and saved as  /tmp/exit1  sha256sum:  67e1a85dd8179c51bc7a230ea849892079c82e35dbe6af46f99d508c3f192008]
2015/06/05 16:54:05 DEBUG iCMRAjWw [Incoming POST request from IP: 127.0.0.1]
2015/06/05 16:54:05 DEBUG iCMRAjWw [Check Command: exit0  File  exit0 successfully uploaded and saved as  /tmp/exit0  sha256sum:  3e9ac084c7a364d5daffe5cce11bc9e0a390aa2ee8cb2b1b25e26007b65be37e]
2015/06/05 16:54:09 DEBUG hTHctcuA [Incoming POST request from IP: 127.0.0.1]
2015/06/05 16:54:09 DEBUG hTHctcuA [Check Command: exit3  File  exit3 successfully uploaded and saved as  /tmp/exit3  sha256sum:  d97fc09d490e28382b25efe85dd8ba53392856c2d90e086e41beceb7a6f6a4bc]
2015/06/05 16:54:13 DEBUG xhxKQFDa [Incoming POST request from IP: 127.0.0.1]
2015/06/05 16:54:13 DEBUG xhxKQFDa [Check Command: exit2  File  exit2 successfully uploaded and saved as  /tmp/exit2  sha256sum:  9303021249cb56ead69ac17d0ea66d32f246e41c2b50efbdc89c427848743024]
```

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

