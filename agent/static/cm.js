var classes = {
  agent: "\u2063\u2063\u2063",
  err: "\u2063\u2063",
  out: "\u2063"
};

CodeMirror.modeURL =
  "//cdnjs.cloudflare.com/ajax/libs/codemirror/5.30.0/mode/%N/%N.min.js";

CodeMirror.defineMode("log", function() {
  return {
    token: function(stream, state) {
      var style = null;
      for (var k in classes) {
        if (stream.match(classes[k])) {
          style = k;
          break;
        }
      }
      if (style) {
        this.prev = style;
        return style;
      }
      stream.next();
      return this.prev;
    }
  };
});
