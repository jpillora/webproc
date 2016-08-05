/* global $,ace */

var editor = ace.edit("editor");
editor.setShowPrintMargin(false);
editor.$blockScrolling = Infinity;
editor.setTheme("ace/theme/github");
// editor.getSession().setMode("ace/mode/toml");
editor.setAutoScrollEditorIntoView(true);

var app = angular.module('webproc', []);

var renderLogger = function(data) {
  var n = renderLogger.n || 0;
  var logger = document.querySelector("#logger");
  while(true) {
    var m = data.Log[n];
    if(!m) break;
    n++;
    if(m.$rendered) continue;
    var span = document.createElement("span");
    span.textContent = m.b;
    span.class = m.p;
    logger.appendChild(span);
    m.$rendered = true;
  }
  renderLogger.n = n;
};

app.run(function($rootScope) {
  var s = window.root = $rootScope;
  var inputs = s.inputs = {};
  //server data
  var url = location.pathname.replace(/[^\/]+$/,"") + "sync";
  var data = s.data = {};
  var v = velox.sse(url, data);
  v.onupdate = function() {
    s.$apply();
    renderLogger(data);
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
    if(s.files.length === 1) {
      inputs.file = s.files[0];
    }
  });
  s.$watch("inputs.file", function(file) {
    if(!file) return;
    var ext = /\.(toml|json|yaml|sh)$/.test(file) ? RegExp.$1 : 'ini';
    editor.getSession().setMode("ace/mode/"+ext);
    updateEditor();
  });
});
