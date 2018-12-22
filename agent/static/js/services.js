app.service("localOpts", function($rootScope) {
  var cache = $rootScope.$new(true);
  return function(key, defaults) {
    var str = localStorage.getItem(key);
    if (!str) {
      str = "{}";
    }
    //set initial value
    cache[key] = angular.extend({}, defaults || {}, JSON.parse(str));
    //watch for changes
    cache.$watch(
      key,
      function(val) {
        localStorage.setItem(key, JSON.stringify(val));
      },
      true
    );
    //return object
    return cache[key];
  };
});
