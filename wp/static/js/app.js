/* global $,ace */

var editor = ace.edit("editor");
editor.setShowPrintMargin(false);
editor.$blockScrolling = Infinity;
editor.setTheme("ace/theme/github");
editor.setAutoScrollEditorIntoView(true);

var app = angular.module('webproc', []);

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
        document.querySelector(".follow.icon").style.display = follow ? 'block' : 'none';
      });
      function followLog() {
        if(scope.follow) e.scrollTop = 99999999;
      }
      angular.element(window).on('resize', followLog);
      scope.$on('update', function(event, data) {
        //bound current index by min log entry
        n = Math.max(n, data.LogOffset-data.LogMaxSize);
        while(true) {
          var m = data.Log[n];
          if(!m) break;
          n++;
          if(m.$rendered) continue;
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

app.directive("ago", function() {
  var scale = [["ms",1000], ["s",60], ["m",60], ["h",24], ["d",31], ["mth",12]];
  var ago = function(str) {
    var v = +new Date()-new Date(str);
    for(var i = 0; i < scale.length; i++) {
      var s = scale[i];
      if(v < s[1]) return v + s[0];
      v = Math.round(v/s[1]);
    }
    return "-";
  };
  return {
    restrict: "A",
    link: function(s, e, attrs) {
      var str;
      var check = function() {
        e.text(ago(str));
        setTimeout(check, 1000);
      }
      check();
      s.$watch(attrs.ago, function(s) {
        str = s;
      });
    }
  }
});

app.run(function($rootScope, $http, $timeout) {
  var s = window.root = $rootScope;
  var inputs = s.inputs = {
    show: {out:true,err:true,agent:false}
  };
  //server data
  var url = location.pathname.replace(/[^\/]+$/,"") + "sync";
  var data = s.data = {};
  var v = velox.sse(url, data);
  s.reconnect = function() {
    v.retry();
  };
  v.onupdate = function() {
    s.$apply();
    s.$emit('update', data);
  };
  v.onchange = function(connected) {
    s.connected = connected;
    s.$apply();
  };
  //put file contents into editor
  var updateEditor = function() {
    var v = data.Files[inputs.file] || "";
    var curr = editor.getValue();
    if(curr !== v)
      editor.setValue(v, -1);
  };
  //handle changes
  s.$watch("data.Config.ConfigurationFiles", function(files) {
    s.files = files || [];
    if(s.files.length === 1 || (s.files.length >= 1 && !inputs.file)) {
      inputs.file = s.files[0];
    }
  });
  s.$watch("inputs.file", function(file) {
    if(!file) return;
    //extensions (choose from https://github.com/ajaxorg/ace/tree/master/lib/ace/mode)
    var mode = /\.(toml|json|js|css|html|go|ya?ml|sh|xml)$/.test(file) ? RegExp.$1 : 'ini';
    //corrections
    if(mode === 'yml') mode = 'yaml';
    if(mode === 'go') mode = 'golang';
    editor.getSession().setMode("ace/mode/"+mode);
    updateEditor();
  });
  //commit change
  s.reload = function() {
    s.reload.ing = true;
    s.reload.err = null;
    $http.post('configure', {
      file: inputs.file,
      contents: editor.getValue()
    }).then(function() {
      s.reload.ed = true;
      $timeout(function() { s.reload.ed = false; }, 5000);
    }, function(resp) {
      s.reload.err = resp.data;
    }).finally(function() {
      s.reload.ing = false;
    });
  };
});
