/* global $,ace */

var editor = ace.edit("editor");
editor.setShowPrintMargin(false);
editor.$blockScrolling = Infinity;
editor.setTheme("ace/theme/github");
editor.setAutoScrollEditorIntoView(true);

var editorSetValue = function(val) {
  var i = editor.session.doc.positionToIndex(editor.selection.getCursor());
  editor.setValue(val, i || -1);
}

var app = angular.module('webproc', []);

var since = (function() {
  var scale = [["ms",1000], ["s",60], ["m",60], ["h",24], ["d",31], ["mth",12]];
  return function(date) {
    var v = +new Date()-date;
    for(var i = 0; i < scale.length; i++) {
      var s = scale[i];
      if(v < s[1]) return v + s[0];
      v = Math.round(v/s[1]);
    }
    return "-";
  };
}());

app.directive("ago", function() {
  return {
    restrict: "A",
    link: function(s, e, attrs) {
      var d, t;
      var check = function() {
        clearTimeout(t);
        if(d) e.text(since(d));
        t = setTimeout(check, 1000);
      }
      s.$watch(attrs.ago, function(s) {
        d = new Date(s);
        check();
      });
    }
  }
});

app.directive("log", function() {
  return {
    restrict: 'C',
    link: function(scope, elem, attrs) {
      var e = window.e = elem[0];
      var n = 0;
      var scroll = 0;
      scope.follow = true;
      elem.on('scroll', function(event) {
        var scrollDiff = (e.scrollHeight - e.clientHeight);
        var percent = scrollDiff == 0 ? 100 : (e.scrollTop/scrollDiff)*100;
        var follow = percent === 100;
        if(follow === scope.follow) return;
        scope.follow = follow;
        document.querySelector(".follow.icon").style.opacity = follow ? 1 : 0;
      });
      function followLog() {
        if(scope.follow) e.scrollTop = 99999999;
      }
      angular.element(window).on('resize', followLog);
      scope.$on('reset', function(event, data) {
        n = 0;
        e.querySelectorAll("span").forEach(function(span) {
          span.remove();
        });
      });
      scope.$on('save', function(event, data) {
        //bound current index by min log entry
        n = Math.max(n, data.LogOffset-data.LogMaxSize);
        while(true) {
          var m = data.Log[n];
          if(!m) break;
          n++;
          var span = document.createElement("span");
          span.textContent = m.b;
          span.className = m.p;
          e.appendChild(span);
          m.$rendered = true;
        }
        followLog();
      });
    }
  }
});

app.service("sync", function() {
  return function(obj, key) {
    var val = localStorage.getItem(key);
    if(val) {
      console.log("load", key, val);
      obj.$eval(key + "=" + val);
    }
    obj.$watch(key, function(val) {
      var str = JSON.stringify(val);
      console.log("set", key, str);
      localStorage.setItem(key, str);
    }, true);
  };
});

app.run(function($rootScope, $http, $timeout, sync) {
  var s = window.root = $rootScope;
  var inputs = s.inputs = {
    show: {
      out: true,
      err: true,
      agent: false
    },
    file: '',
    files: null
  };
  sync($rootScope, "inputs.show");
  //server data
  var url = location.pathname.replace(/[^\/]+$/,"") + "sync";
  var data = s.data = {};
  var v = s.v = velox.sse(url, data);
  s.reconnect = function() {
    v.retry();
  };
  var id = "";
  v.onupdate = function() {
    if(v.id !== id) {
      id = v.id;
      s.$emit('reset');
    }
    s.$emit('save', data);
    s.$apply();
  };
  v.onchange = function(connected) {
    s.connected = connected;
    s.$apply();
  };
  //
  s.saved = true;
  var checkSaved = function() {
    if(!inputs.file || !inputs.files) {
      s.saved = true;
      return;
    }
    var client = inputs.files && inputs.files[inputs.file];
    var server = data.Files[inputs.file];
    s.saved = client === server;
  };
  //editor changes
  editor.on("input", function() {
    //cache current
    inputs.files[inputs.file] = editor.getValue();
    checkSaved();
    s.$apply();
  })
  //handle changes
  s.$watch("data.Config.ConfigurationFiles", function(files) {
    s.files = files || [];
    if(s.files.length === 1 || (s.files.length >= 1 && !inputs.file)) {
      inputs.file = s.files[0];
    }
  }, true);
  s.$watch("data.Files", function(files) {
    //apply intial file inputs
    if(files && !inputs.files)
      inputs.files = angular.copy(files);
    checkSaved();
  }, true);
  s.$watch("inputs.file", function(file) {
    if(!file) return;
    //extensions (choose from https://github.com/ajaxorg/ace/tree/master/lib/ace/mode)
    var mode = /\.(toml|json|js|css|html|go|ya?ml|sh|xml)$/.test(file) ? RegExp.$1 : 'ini';
    //corrections
    if(mode === 'yml') mode = 'yaml';
    if(mode === 'go') mode = 'golang';
    editor.getSession().setMode("ace/mode/"+mode);
    //load file from cache
    var v = inputs.files[inputs.file] || "";
    var curr = editor.getValue();
    if(curr !== v)
      editorSetValue(v);
    //check if saved
    checkSaved();
  });
  //start/restart
  s.start = function() {
    var alreadyRunning = data.Running;
    s.start.ing = true;
    s.start.err = null;
    $http.put('restart').then(function() {
      s.start.ed = alreadyRunning ? 'Restarted' : 'Started';
      $timeout(function() { s.start.ed = false; }, 3000);
    }, function(resp) {
      s.start.err = resp.data;
    }).finally(function() {
      s.start.ing = false;
    });
  };
  //commit change
  s.save = function() {
    s.save.ing = true;
    s.save.err = null;
    $http.post('save', inputs.files).then(function() {
      s.save.ed = true;
      $timeout(function() { s.save.ed = false; }, 3000);
    }, function(resp) {
      s.save.err = resp.data;
    }).finally(function() {
      s.save.ing = false;
    });
  };

  s.revert = function() {
    editorSetValue(data.Files[inputs.file]);
    checkSaved();
  };
});
