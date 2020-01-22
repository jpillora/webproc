app.run(function($rootScope, $http, $timeout, localOpts) {
  var s = (window.root = $rootScope);
  s.title = "webproc";
  //issue a refresh on app load
  $http.put("refresh");
  //editor states will be stored here
  s.cfg = {
    opts: {
      theme: "eclipse",
      lineNumbers: true
    }
  };
  s.log = {
    mode: "log",
    followLock: true,
    opts: {
      theme: "eclipse",
      readOnly: true,
      lineWrapping: true
    }
  };
  var inputs = {
    show: localOpts("shown", {
      out: true,
      err: true,
      agent: false
    }),
    file: "",
    files: null
  };
  s.inputs = inputs;
  //server data
  var data = (s.data = {});
  //===================================
  var currId = 0;
  var renderLog = function(delta) {
    if (!data.Log) {
      return "";
    }
    if (!delta) {
      currId = 0;
    }
    //bound current index by min log entry
    currId = Math.max(currId, data.LogOffset - data.LogMaxSize);
    //collect new/selected lines
    var lines = [];
    while (true) {
      var m = data.Log[currId];
      if (!m) {
        break;
      }
      currId++;
      if (inputs.show[m.p]) {
        lines.push(classes[m.p] + m.b);
      }
    }
    return lines.join("");
  };
  var renderFull = function() {
    s.log.set(renderLog(false).replace(/\n$/, ""));
  };
  var renderDelta = function() {
    s.log.append("\n" + renderLog(true).replace(/\n$/, ""));
  };
  //===================================
  var url = location.pathname.replace(/[^\/]+$/, "") + "sync";
  var v = (s.v = velox.sse(url, data));
  s.reconnect = function() {
    v.retry();
  };
  var id = "";
  v.onupdate = function() {
    if (v.id === id) {
      renderDelta();
    } else {
      renderFull();
    }
    id = v.id;
    s.$apply();
  };
  v.onchange = function(connected) {
    s.connected = connected;
    s.$apply();
    new Favico({
      fontFamily: "Icons",
      bgColor: connected ? "#21BA45" : "#DB2828"
    }).badge("\uf0e7");
  };
  //compare client config against server config
  s.saved = true;
  var checkSaved = function() {
    if (!inputs.file || !inputs.files) {
      s.saved = true;
      return;
    }
    var client = inputs.files && inputs.files[inputs.file];
    var server = data.Files[inputs.file];
    s.saved = client === server;
  };
  //editor changes
  s.cfg.onchange = function() {
    //cache current
    inputs.files[inputs.file] = s.cfg.get();
    checkSaved();
    s.$apply();
  };
  //handle changes
  s.$watch("data.Config.ProgramArgs", function(args) {
    if (!args) {
      return;
    }
    var prog = args[0];
    if (!prog) {
      return;
    }
    if (/([^\/]+)$/.test(prog)) {
      prog = RegExp.$1;
    }
    s.title = prog;
  });

  s.$watch(
    "data.Config.ConfigurationFiles",
    function(files) {
      //received changes to config files
      s.files = files || [];
      if (s.files.length === 1 || (s.files.length >= 1 && !inputs.file)) {
        inputs.file = s.files[0];
      }
    },
    true
  );
  s.$watch(
    "data.Files",
    function(files) {
      //apply intial file inputs
      if (files && !inputs.files) {
        inputs.files = angular.copy(files);
      }
      s.revert();
    },
    true
  );
  s.$watch("inputs.file", function(file) {
    if (!file) return;
    //extensions
    var mode = "properties";
    if (/.+\.(.+)$/.test(file)) {
      var ext = RegExp.$1;
      var info = CodeMirror.findModeByExtension(ext);
      if (info && info.mode !== "null") {
        mode = info.mode;
      }
    }
    s.cfg.mode(mode);
    //load file from cache
    var v = inputs.files[inputs.file] || "";
    var curr = s.cfg.get();
    if (curr !== v) {
      s.cfg.set(v);
    }
    //check if saved
    checkSaved();
  });

  s.$watch(
    "inputs.show",
    function() {
      if (s.log) {
        renderFull();
      }
    },
    true
  );

  //start/restart
  s.start = function() {
    var alreadyRunning = data.Running;
    s.start.ing = true;
    s.start.err = null;
    $http
      .put("restart")
      .then(
        function() {
          s.start.ed = alreadyRunning ? "Restarted" : "Started";
          $timeout(function() {
            s.start.ed = false;
          }, 3000);
        },
        function(resp) {
          s.start.err = resp.data;
        }
      )
      .finally(function() {
        s.start.ing = false;
      });
  };
  //commit change
  s.save = function() {
    s.save.ing = true;
    s.save.err = null;
    currentFile = {};
    currentFile[inputs.file] = inputs.files[inputs.file];
    $http
      .post("save", currentFile)
      .then(
        function() {
          s.save.ed = true;
          $timeout(function() {
            s.save.ed = false;
          }, 3000);
        },
        function(resp) {
          s.save.err = resp.data;
        }
      )
      .finally(function() {
        s.save.ing = false;
      });
  };

  s.revert = function() {
    if (data.Files && inputs.file in data.Files) {
      s.cfg.set(data.Files[inputs.file]);
    }
    checkSaved();
  };
});
