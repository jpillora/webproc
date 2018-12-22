app.directive("ago", function() {
  var since = (function() {
    var scale = [
      ["ms", 1000],
      ["s", 60],
      ["m", 60],
      ["h", 24],
      ["d", 31],
      ["mth", 12]
    ];
    return function(date) {
      var v = +new Date() - date;
      for (var i = 0; i < scale.length; i++) {
        var s = scale[i];
        if (v < s[1]) return v + s[0];
        v = Math.round(v / s[1]);
      }
      return "-";
    };
  })();

  return {
    restrict: "A",
    link: function(s, e, attrs) {
      var d, t;
      var check = function() {
        clearTimeout(t);
        if (d) e.text(since(d));
        t = setTimeout(check, 1000);
      };
      s.$watch(attrs.ago, function(s) {
        d = new Date(s);
        check();
      });
    }
  };
});

app.directive("cmContainer", function($rootScope) {
  return {
    restrict: "C",
    link: function(scope, jq, attrs) {
      var elem = jq[0];
      var name = attrs.name;
      if (!name) {
        throw "no name";
      }
      var api = $rootScope[name];
      if (!api) {
        throw "api not there";
      }
      var opts = angular.extend({viewportMargin: Infinity}, api.opts);
      var editor = CodeMirror(elem, opts);
      window["cm" + name] = api;
      //optional handler
      if (api.onchange) {
        editor.doc.on("change", function() {
          api.onchange();
        });
      }
      var initialMode = api.mode || null;
      //code mirror api
      api.set = function(val) {
        window.requestAnimationFrame(function() {
          editor.setValue(val || "");
          api.followScroll();
        });
      };
      api.get = function() {
        return editor.getValue();
      };
      api.append = function(line) {
        editor.replaceRange(line, CodeMirror.Pos(editor.lastLine()));
        api.followScroll();
      };
      api.mode = function(mode) {
        editor.setOption("mode", mode);
        CodeMirror.autoLoadMode(editor, mode);
      };
      if (initialMode) {
        api.mode(initialMode);
      }
      if (api.followLock) {
        api.following = true;
        api.followScroll = function() {
          if (api.following) {
            root.log.editor.doc.setSelection({
              line: root.log.editor.doc.lineCount(),
              ch: 0
            });
          }
        };
        api.followCheck = function() {
          var info = editor.getScrollInfo();
          var scrollh = elem.clientHeight + info.top;
          var p = scrollh / info.height;
          var following = p >= 0.95;
          if (following !== api.following) {
            console.log("follow", name, following);
          }
          api.following = following;
          $rootScope.$applyAsync();
        };

        editor.on("scroll", api.followCheck);
      } else {
        api.followScroll = function() {
          //noop
        };
      }

      scroll;
      var followLock = false;
      api.follow = function(f) {
        follow = f;
        //on scroll, detect if bottom, if so following=true
        //on append, if following, scroll bottom
      };
      api.editor = editor;
    }
  };
});
