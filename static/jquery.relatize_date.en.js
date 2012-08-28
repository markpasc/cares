if(typeof $relatizeDateTranslation == "undefined") 
  $relatizeDateTranslation = {};

var _da = ' days ago';
$relatizeDateTranslation.en = {
  default_time_fmt: 12,
  shortDays: [ 'Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat' ],
  days: ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'],
  shortMonths: [ 'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec' ],
  months: [ 'January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December' ],
  ltm: "less than a minute ago",
  abm: "about a minute ago",
  m: "%d minutes ago",
  h: "about an hour ago",
  abh: "about %d hours ago",
  d: "yesterday",
  at: "at",
  ds: "%d"+_da,
  shortds: ['two'+_da, 'three'+_da, 'four'+_da, 'five'+_da]
};