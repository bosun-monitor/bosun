/// <reference path="angular.d.ts" />
/// <reference path="angular-route.d.ts" />
/// <reference path="angular-sanitize.d.ts" />
/// <reference path="bootstrap.d.ts" />
/// <reference path="moment.d.ts" />
/// <reference path="moment-duration-format.d.ts" />
/// <reference path="d3.d.ts" />
/// <reference path="underscore.d.ts" />
var bosunApp = angular.module('bosunApp', [
    'ngRoute',
    'bosunControllers',
    'mgcrea.ngStrap',
    'ngSanitize',
    'ui.ace',
]);
bosunApp.config(['$routeProvider', '$locationProvider', '$httpProvider', function ($routeProvider, $locationProvider, $httpProvider) {
        $locationProvider.html5Mode({
            enabled: true,
            requireBase: false
        });
        $routeProvider.
            when('/', {
            title: 'Dashboard',
            templateUrl: 'partials/dashboard.html',
            controller: 'DashboardCtrl'
        }).
            when('/items', {
            title: 'Items',
            templateUrl: 'partials/items.html',
            controller: 'ItemsCtrl'
        }).
            when('/expr', {
            title: 'Expression',
            templateUrl: 'partials/expr.html',
            controller: 'ExprCtrl'
        }).
            when('/errors', {
            title: 'Errors',
            templateUrl: 'partials/errors.html',
            controller: 'ErrorCtrl'
        }).
            when('/graph', {
            title: 'Graph',
            templateUrl: 'partials/graph.html',
            controller: 'GraphCtrl'
        }).
            when('/host', {
            title: 'Host View',
            templateUrl: 'partials/host.html',
            controller: 'HostCtrl',
            reloadOnSearch: false
        }).
            when('/silence', {
            title: 'Silence',
            templateUrl: 'partials/silence.html',
            controller: 'SilenceCtrl'
        }).
            when('/config', {
            title: 'Configuration',
            templateUrl: 'partials/config.html',
            controller: 'ConfigCtrl',
            reloadOnSearch: false
        }).
            when('/action', {
            title: 'Action',
            templateUrl: 'partials/action.html',
            controller: 'ActionCtrl'
        }).
            when('/history', {
            title: 'Alert History',
            templateUrl: 'partials/history.html',
            controller: 'HistoryCtrl'
        }).
            when('/put', {
            title: 'Data Entry',
            templateUrl: 'partials/put.html',
            controller: 'PutCtrl'
        }).
            when('/annotation', {
            title: 'Annotation',
            templateUrl: 'partials/annotation.html',
            controller: 'AnnotationCtrl'
        }).
            when('/incident', {
            title: 'Incident',
            templateUrl: 'partials/incident.html',
            controller: 'IncidentCtrl'
        }).
            otherwise({
            redirectTo: '/'
        });
        $httpProvider.interceptors.push(function ($q) {
            return {
                'request': function (config) {
                    config.headers['X-Miniprofiler'] = 'true';
                    return config;
                }
            };
        });
    }]);
bosunApp.run(['$location', '$rootScope', function ($location, $rootScope) {
        $rootScope.$on('$routeChangeSuccess', function (event, current, previous) {
            $rootScope.title = current.$$route.title;
            $rootScope.shortlink = false;
        });
    }]);
var bosunControllers = angular.module('bosunControllers', []);
bosunControllers.controller('BosunCtrl', ['$scope', '$route', '$http', '$q', '$rootScope', function ($scope, $route, $http, $q, $rootScope) {
        $scope.$on('$routeChangeSuccess', function (event, current, previous) {
            $scope.stop(true);
        });
        $scope.active = function (v) {
            if (!$route.current) {
                return null;
            }
            if ($route.current.loadedTemplateUrl == 'partials/' + v + '.html') {
                return { active: true };
            }
            return null;
        };
        $http.get("/api/annotate")
            .success(function (data) {
            $scope.annotateEnabled = data;
        })
            .error(function (data) {
            console.log(data);
        });
        $http.get("/api/quiet")
            .success(function (data) {
            $scope.quiet = data;
        })
            .error(function (data) {
            console.log(data);
        });
        $http.get("/api/opentsdb/version")
            .success(function (data) {
            $scope.version = data;
            $scope.opentsdbEnabled = $scope.version.Major != 0 && $scope.version.Minor != 0;
        })
            .error(function (data) {
            console.log(data);
        });
        ;
        $scope.json = function (v) {
            return JSON.stringify(v, null, '  ');
        };
        $scope.btoa = function (v) {
            return encodeURIComponent(btoa(v));
        };
        $scope.encode = function (v) {
            return encodeURIComponent(v);
        };
        $scope.req_from_m = function (m) {
            var r = new Request();
            var q = new Query(false);
            q.metric = m;
            r.queries.push(q);
            return r;
        };
        $scope.panelClass = function (status, prefix) {
            if (prefix === void 0) { prefix = "panel-"; }
            switch (status) {
                case "critical": return prefix + "danger";
                case "unknown": return prefix + "info";
                case "warning": return prefix + "warning";
                case "normal": return prefix + "success";
                case "error": return prefix + "danger";
                default: return prefix + "default";
            }
        };
        $scope.values = {};
        $scope.setKey = function (key, value) {
            if (value === undefined) {
                delete $scope.values[key];
            }
            else {
                $scope.values[key] = value;
            }
        };
        $scope.getKey = function (key) {
            return $scope.values[key];
        };
        var scheduleFilter;
        $scope.refresh = function (filter) {
            var d = $q.defer();
            scheduleFilter = filter;
            $scope.animate();
            var p = $http.get('/api/alerts?filter=' + encodeURIComponent(filter || ""))
                .success(function (data) {
                $scope.schedule = data;
                $scope.timeanddate = data.TimeAndDate;
                d.resolve();
            })
                .error(function (err) {
                d.reject(err);
            });
            p.finally($scope.stop);
            return d.promise;
        };
        var sz = 30;
        var orig = 700;
        var light = '#4ba2d9';
        var dark = '#1f5296';
        var med = '#356eb6';
        var mult = sz / orig;
        var bgrad = 25 * mult;
        var circles = [
            [150, 150, dark],
            [550, 150, dark],
            [150, 550, light],
            [550, 550, light],
        ];
        var svg = d3.select('#logo')
            .append('svg')
            .attr('height', sz)
            .attr('width', sz);
        svg.selectAll('rect.bg')
            .data([[0, light], [sz / 2, dark]])
            .enter()
            .append('rect')
            .attr('class', 'bg')
            .attr('width', sz)
            .attr('height', sz / 2)
            .attr('rx', bgrad)
            .attr('ry', bgrad)
            .attr('fill', function (d) { return d[1]; })
            .attr('y', function (d) { return d[0]; });
        svg.selectAll('path.diamond')
            .data([150, 550])
            .enter()
            .append('path')
            .attr('d', function (d) {
            var s = 'M ' + d * mult + ' ' + 150 * mult;
            var w = 200 * mult;
            s += ' l ' + w + ' ' + w;
            s += ' l ' + -w + ' ' + w;
            s += ' l ' + -w + ' ' + -w + ' Z';
            return s;
        })
            .attr('fill', med)
            .attr('stroke', 'white')
            .attr('stroke-width', 15 * mult);
        svg.selectAll('rect.white')
            .data([150, 350, 550])
            .enter()
            .append('rect')
            .attr('class', 'white')
            .attr('width', .5)
            .attr('height', '100%')
            .attr('fill', 'white')
            .attr('x', function (d) { return d * mult; });
        svg.selectAll('circle')
            .data(circles)
            .enter()
            .append('circle')
            .attr('cx', function (d) { return d[0] * mult; })
            .attr('cy', function (d) { return d[1] * mult; })
            .attr('r', 62.5 * mult)
            .attr('fill', function (d) { return d[2]; })
            .attr('stroke', 'white')
            .attr('stroke-width', 25 * mult);
        var transitionDuration = 750;
        var animateCount = 0;
        $scope.animate = function () {
            animateCount++;
            if (animateCount == 1) {
                doAnimate();
            }
        };
        function doAnimate() {
            if (!animateCount) {
                return;
            }
            d3.shuffle(circles);
            svg.selectAll('circle')
                .data(circles, function (d, i) { return i; })
                .transition()
                .duration(transitionDuration)
                .attr('cx', function (d) { return d[0] * mult; })
                .attr('cy', function (d) { return d[1] * mult; })
                .attr('fill', function (d) { return d[2]; });
            setTimeout(doAnimate, transitionDuration);
        }
        $scope.stop = function (all) {
            if (all === void 0) { all = false; }
            if (all) {
                animateCount = 0;
            }
            else if (animateCount > 0) {
                animateCount--;
            }
        };
        var short = $('#shortlink')[0];
        $scope.shorten = function () {
            $http.get('/api/shorten').success(function (data) {
                if (data.id) {
                    short.value = data.id;
                    $rootScope.shortlink = true;
                    setTimeout(function () {
                        short.setSelectionRange(0, data.id.length);
                    });
                }
            });
        };
    }]);
var tsdbDateFormat = 'YYYY/MM/DD-HH:mm:ss';
moment.defaultFormat = tsdbDateFormat;
moment.locale('en', {
    relativeTime: {
        future: "in %s",
        past: "%s-ago",
        s: "%ds",
        m: "%dm",
        mm: "%dm",
        h: "%dh",
        hh: "%dh",
        d: "%dd",
        dd: "%dd",
        M: "%dn",
        MM: "%dn",
        y: "%dy",
        yy: "%dy"
    }
});
function ruleUrl(ak, fromTime) {
    var openBrack = ak.indexOf("{");
    var closeBrack = ak.indexOf("}");
    var alertName = ak.substr(0, openBrack);
    var template = ak.substring(openBrack + 1, closeBrack);
    var url = '/api/rule?' +
        'alert=' + encodeURIComponent(alertName) +
        '&from=' + encodeURIComponent(fromTime.format()) +
        '&template_group=' + encodeURIComponent(template);
    return url;
}
function configUrl(ak, fromTime) {
    var openBrack = ak.indexOf("{");
    var closeBrack = ak.indexOf("}");
    var alertName = ak.substr(0, openBrack);
    var template = ak.substring(openBrack + 1, closeBrack);
    // http://bosun/config?alert=haproxy.server.downtime.ny&fromDate=2016-07-10&fromTime=21%3A03
    var url = '/config?' +
        'alert=' + encodeURIComponent(alertName) +
        '&fromDate=' + encodeURIComponent(fromTime.format("YYYY-MM-DD")) +
        '&fromTime=' + encodeURIComponent(fromTime.format("HH:mm"));
    return url;
}
function createCookie(name, value, days) {
    var expires;
    if (days) {
        var date = new Date();
        date.setTime(date.getTime() + (days * 24 * 60 * 60 * 1000));
        expires = "; expires=" + date.toGMTString();
    }
    else {
        expires = "";
    }
    document.cookie = escape(name) + "=" + escape(value) + expires + "; path=/";
}
function readCookie(name) {
    var nameEQ = escape(name) + "=";
    var ca = document.cookie.split(';');
    for (var i = 0; i < ca.length; i++) {
        var c = ca[i];
        while (c.charAt(0) === ' ')
            c = c.substring(1, c.length);
        if (c.indexOf(nameEQ) === 0)
            return unescape(c.substring(nameEQ.length, c.length));
    }
    return null;
}
function eraseCookie(name) {
    createCookie(name, "", -1);
}
function getUser() {
    return readCookie('action-user');
}
function setUser(name) {
    createCookie('action-user', name, 1000);
}
function getOwner() {
    return readCookie('action-owner');
}
function setOwner(name) {
    createCookie('action-owner', name, 1000);
}
function getShowAnnotations() {
    return readCookie('annotations-show');
}
function setShowAnnotations(yes) {
    createCookie('annotations-show', yes, 1000);
}
// from: http://stackoverflow.com/a/15267754/864236
bosunApp.filter('reverse', function () {
    return function (items) {
        if (!angular.isArray(items)) {
            return [];
        }
        return items.slice().reverse();
    };
});
var timeFormat = 'YYYY-MM-DDTHH:mm:ssZ';
var Annotation = (function () {
    function Annotation(a, get) {
        a = a || {};
        this.Id = a.Id || "";
        this.Message = a.Message || "";
        this.StartDate = a.StartDate || "";
        this.EndDate = a.EndDate || "";
        this.CreationUser = a.CreationUser || !get && getUser() || "";
        this.Url = a.Url || "";
        this.Source = a.Source || "bosun-ui";
        this.Host = a.Host || "";
        this.Owner = a.Owner || !get && getOwner() || "";
        this.Category = a.Category || "";
    }
    Annotation.prototype.setTimeUTC = function () {
        var now = moment().utc().format(timeFormat);
        this.StartDate = now;
        this.EndDate = now;
    };
    Annotation.prototype.setTime = function () {
        var now = moment().format(timeFormat);
        this.StartDate = now;
        this.EndDate = now;
    };
    return Annotation;
})();
bosunControllers.controller('ActionCtrl', ['$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        var search = $location.search();
        $scope.user = readCookie("action-user");
        $scope.type = search.type;
        $scope.notify = true;
        $scope.msgValid = true;
        $scope.message = "";
        $scope.validateMsg = function () {
            $scope.msgValid = (!$scope.notify) || ($scope.message != "");
        };
        if (search.key) {
            var keys = search.key;
            if (!angular.isArray(search.key)) {
                keys = [search.key];
            }
            $location.search('key', null);
            $scope.setKey('action-keys', keys);
        }
        else {
            $scope.keys = $scope.getKey('action-keys');
        }
        $scope.submit = function () {
            $scope.validateMsg();
            if (!$scope.msgValid || ($scope.user == "")) {
                return;
            }
            var data = {
                Type: $scope.type,
                User: $scope.user,
                Message: $scope.message,
                Keys: $scope.keys,
                Notify: $scope.notify
            };
            createCookie("action-user", $scope.user, 1000);
            $http.post('/api/action', data)
                .success(function (data) {
                $location.url('/');
            })
                .error(function (error) {
                alert(error);
            });
        };
    }]);
bosunControllers.controller('AnnotationCtrl', ['$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        var search = $location.search();
        $scope.id = search.id;
        if ($scope.id && $scope.id != "") {
            $http.get('/api/annotation/' + $scope.id)
                .success(function (data) {
                $scope.annotation = new Annotation(data, true);
                $scope.error = "";
            })
                .error(function (data) {
                $scope.error = "failed to get annotation with id: " + $scope.id + ", error: " + data;
            });
        }
        else {
            $scope.annotation = new Annotation();
            $scope.annotation.setTimeUTC();
        }
        $http.get('/api/annotation/values/Owner')
            .success(function (data) {
            $scope.owners = data;
        });
        $http.get('/api/annotation/values/Category')
            .success(function (data) {
            $scope.categories = data;
        });
        $http.get('/api/annotation/values/Host')
            .success(function (data) {
            $scope.hosts = data;
        });
        $scope.submitAnnotation = function () { return $http.post('/api/annotation', $scope.annotation)
            .success(function (data) {
            $scope.annotation = new Annotation(data, true);
            $scope.error = "";
            $scope.submitSuccess = true;
            $scope.deleteSuccess = false;
        })
            .error(function (error) {
            $scope.error = error;
            $scope.submitSuccess = false;
        }); };
        $scope.deleteAnnotation = function () { return $http.delete('/api/annotation/' + $scope.annotation.Id)
            .success(function (data) {
            $scope.error = "";
            $scope.deleteSuccess = true;
            $scope.submitSuccess = false;
            $scope.annotation = new (Annotation);
            $scope.annotation.setTimeUTC();
        })
            .error(function (error) {
            $scope.error = "failed to delete annotation with id: " + $scope.annotation.Id + ", error: " + error;
            $scope.deleteSuccess = false;
        }); };
    }]);
bosunControllers.controller('ConfigCtrl', ['$scope', '$http', '$location', '$route', '$timeout', '$sce', function ($scope, $http, $location, $route, $timeout, $sce) {
        var search = $location.search();
        $scope.fromDate = search.fromDate || '';
        $scope.fromTime = search.fromTime || '';
        $scope.toDate = search.toDate || '';
        $scope.toTime = search.toTime || '';
        $scope.intervals = +search.intervals || 5;
        $scope.duration = +search.duration || null;
        $scope.config_text = 'Loading config...';
        $scope.selected_alert = search.alert || '';
        $scope.email = search.email || '';
        $scope.template_group = search.template_group || '';
        $scope.items = parseItems();
        $scope.tab = search.tab || 'results';
        $scope.aceTheme = 'chrome';
        $scope.aceMode = 'bosun';
        var expr = search.expr;
        function buildAlertFromExpr() {
            if (!expr)
                return;
            var newAlertName = "test";
            var idx = 1;
            //find a unique alert name
            while ($scope.items["alert"].indexOf(newAlertName) != -1 || $scope.items["template"].indexOf(newAlertName) != -1) {
                newAlertName = "test" + idx;
                idx++;
            }
            var text = '\n\ntemplate ' + newAlertName + ' {\n' +
                '	subject = {{.Last.Status}}: {{.Alert.Name}} on {{.Group.host}}\n' +
                '	body = `<p>Name: {{.Alert.Name}}\n' +
                '	<p>Tags:\n' +
                '	<table>\n' +
                '		{{range $k, $v := .Group}}\n' +
                '			<tr><td>{{$k}}</td><td>{{$v}}</td></tr>\n' +
                '		{{end}}\n' +
                '	</table>`\n' +
                '}\n\n';
            var expression = atob(expr);
            var lines = expression.split("\n").map(function (l) { return l.trim(); });
            lines[lines.length - 1] = "crit = " + lines[lines.length - 1];
            expression = lines.join("\n    ");
            text += 'alert ' + newAlertName + ' {\n' +
                '	template = ' + newAlertName + '\n' +
                '	' + expression + '\n' +
                '}\n';
            $scope.config_text += text;
            $scope.items = parseItems();
            $timeout(function () {
                //can't scroll editor until after control is updated. Defer it.
                $scope.scrollTo("alert", newAlertName);
            });
        }
        function parseItems() {
            var configText = $scope.config_text;
            var re = /^\s*(alert|template|notification|lookup|macro)\s+([\w\-\.\$]+)\s*\{/gm;
            var match;
            var items = {};
            items["alert"] = [];
            items["template"] = [];
            items["lookup"] = [];
            items["notification"] = [];
            items["macro"] = [];
            while (match = re.exec(configText)) {
                var type = match[1];
                var name = match[2];
                var list = items[type];
                if (!list) {
                    list = [];
                    items[type] = list;
                }
                list.push(name);
            }
            return items;
        }
        $http.get('/api/config?hash=' + (search.hash || ''))
            .success(function (data) {
            $scope.config_text = data;
            $scope.items = parseItems();
            buildAlertFromExpr();
            if (!$scope.selected_alert && $scope.items["alert"].length) {
                $scope.selected_alert = $scope.items["alert"][0];
            }
            $timeout(function () {
                //can't scroll editor until after control is updated. Defer it.
                $scope.scrollTo("alert", $scope.selected_alert);
            });
        })
            .error(function (data) {
            $scope.validationResult = "Error fetching config: " + data;
        });
        $scope.reparse = function () {
            $scope.items = parseItems();
        };
        var editor;
        $scope.aceLoaded = function (_editor) {
            editor = _editor;
            $scope.editor = editor;
            editor.getSession().setUseWrapMode(true);
            editor.on("blur", function () {
                $scope.$apply(function () {
                    $scope.items = parseItems();
                });
            });
        };
        var syntax = true;
        $scope.aceToggleHighlight = function () {
            if (syntax) {
                editor.getSession().setMode();
                syntax = false;
                return;
            }
            syntax = true;
            editor.getSession().setMode({
                path: 'ace/mode/' + $scope.aceMode,
                v: Date.now()
            });
        };
        $scope.scrollTo = function (type, name) {
            var searchRegex = new RegExp("^\\s*" + type + "\\s+" + name, "g");
            editor.find(searchRegex, {
                backwards: false,
                wrap: true,
                caseSensitive: false,
                wholeWord: false,
                regExp: true
            });
            if (type == "alert") {
                $scope.selectAlert(name);
            }
        };
        $scope.scrollToInterval = function (id) {
            document.getElementById('time-' + id).scrollIntoView();
            $scope.show($scope.sets[id]);
        };
        $scope.show = function (set) {
            set.show = 'loading...';
            $scope.animate();
            var url = '/api/rule?' +
                'alert=' + encodeURIComponent($scope.selected_alert) +
                '&from=' + encodeURIComponent(set.Time);
            $http.post(url, $scope.config_text)
                .success(function (data) {
                procResults(data);
                set.Results = data.Sets[0].Results;
            })
                .error(function (error) {
                $scope.error = error;
            })
                .finally(function () {
                $scope.stop();
                delete (set.show);
            });
        };
        $scope.setInterval = function () {
            var from = moment.utc($scope.fromDate + ' ' + $scope.fromTime);
            var to = moment.utc($scope.toDate + ' ' + $scope.toTime);
            if (!from.isValid() || !to.isValid()) {
                return;
            }
            var diff = from.diff(to);
            if (!diff) {
                return;
            }
            var intervals = +$scope.intervals;
            if (intervals < 2) {
                return;
            }
            diff /= 1000 * 60;
            var d = Math.abs(Math.round(diff / intervals));
            if (d < 1) {
                d = 1;
            }
            $scope.duration = d;
        };
        $scope.setDuration = function () {
            var from = moment.utc($scope.fromDate + ' ' + $scope.fromTime);
            var to = moment.utc($scope.toDate + ' ' + $scope.toTime);
            if (!from.isValid() || !to.isValid()) {
                return;
            }
            var diff = from.diff(to);
            if (!diff) {
                return;
            }
            var duration = +$scope.duration;
            if (duration < 1) {
                return;
            }
            $scope.intervals = Math.abs(Math.round(diff / duration / 1000 / 60));
        };
        $scope.selectAlert = function (alert) {
            $scope.selected_alert = alert;
            $location.search("alert", alert);
            // Attempt to find `template = foo` in order to set up quick jump between template and alert
            var searchRegex = new RegExp("^\\s*alert\\s+" + alert, "g");
            var lines = $scope.config_text.split("\n");
            $scope.quickJumpTarget = null;
            for (var i = 0; i < lines.length; i++) {
                if (searchRegex.test(lines[i])) {
                    for (var j = i + 1; j < lines.length; j++) {
                        // Close bracket at start of line means end of alert.
                        if (/^\s*\}/m.test(lines[j])) {
                            return;
                        }
                        var found = /^\s*template\s*=\s*([\w\-\.\$]+)/m.exec(lines[j]);
                        if (found) {
                            $scope.quickJumpTarget = "template " + found[1];
                        }
                    }
                }
            }
        };
        $scope.quickJump = function () {
            var parts = $scope.quickJumpTarget.split(" ");
            if (parts.length != 2) {
                return;
            }
            $scope.scrollTo(parts[0], parts[1]);
            if (parts[0] == "template" && $scope.selected_alert) {
                $scope.quickJumpTarget = "alert " + $scope.selected_alert;
            }
        };
        $scope.setTemplateGroup = function (group) {
            var match = group.match(/{(.*)}/);
            if (match) {
                $scope.template_group = match[1];
            }
        };
        var line_re = /test:(\d+)/;
        $scope.validate = function () {
            $http.post('/api/config_test', $scope.config_text)
                .success(function (data) {
                if (data == "") {
                    $scope.validationResult = "Valid";
                    $timeout(function () {
                        $scope.validationResult = "";
                    }, 2000);
                }
                else {
                    $scope.validationResult = data;
                    var m = data.match(line_re);
                    if (angular.isArray(m) && (m.length > 1)) {
                        editor.gotoLine(m[1]);
                    }
                }
            })
                .error(function (error) {
                $scope.validationResult = 'Error validating: ' + error;
            });
        };
        $scope.test = function () {
            $scope.error = '';
            $scope.running = true;
            $scope.warning = [];
            $location.search('fromDate', $scope.fromDate || null);
            $location.search('fromTime', $scope.fromTime || null);
            $location.search('toDate', $scope.toDate || null);
            $location.search('toTime', $scope.toTime || null);
            $location.search('intervals', String($scope.intervals) || null);
            $location.search('duration', String($scope.duration) || null);
            $location.search('email', $scope.email || null);
            $location.search('template_group', $scope.template_group || null);
            $scope.animate();
            var from = moment.utc($scope.fromDate + ' ' + $scope.fromTime);
            var to = moment.utc($scope.toDate + ' ' + $scope.toTime);
            if (!from.isValid()) {
                from = to;
            }
            if (!to.isValid()) {
                to = from;
            }
            if (!from.isValid() && !to.isValid()) {
                from = to = moment.utc();
            }
            var diff = from.diff(to);
            var intervals;
            if (diff == 0) {
                intervals = 1;
            }
            else if (Math.abs(diff) < 60 * 1000) {
                intervals = 2;
            }
            else {
                intervals = +$scope.intervals;
            }
            var url = '/api/rule?' +
                'alert=' + encodeURIComponent($scope.selected_alert) +
                '&from=' + encodeURIComponent(from.format()) +
                '&to=' + encodeURIComponent(to.format()) +
                '&intervals=' + encodeURIComponent(intervals) +
                '&email=' + encodeURIComponent($scope.email) +
                '&template_group=' + encodeURIComponent($scope.template_group);
            $http.post(url, $scope.config_text)
                .success(function (data) {
                $scope.sets = data.Sets;
                $scope.alert_history = data.AlertHistory;
                if (data.Hash) {
                    $location.search('hash', data.Hash);
                }
                procResults(data);
            })
                .error(function (error) {
                $scope.error = error;
            })
                .finally(function () {
                $scope.running = false;
                $scope.stop();
            });
        };
        $scope.zws = function (v) {
            return v.replace(/([,{}()])/g, '$1\u200b');
        };
        $scope.loadTimelinePanel = function (entry, v) {
            if (v.doneLoading && !v.error) {
                return;
            }
            v.error = null;
            v.doneLoading = false;
            var ak = entry.key;
            var openBrack = ak.indexOf("{");
            var closeBrack = ak.indexOf("}");
            var alertName = ak.substr(0, openBrack);
            var template = ak.substring(openBrack + 1, closeBrack);
            var url = '/api/rule?' +
                'alert=' + encodeURIComponent(alertName) +
                '&from=' + encodeURIComponent(moment.utc(v.Time).format()) +
                '&template_group=' + encodeURIComponent(template);
            $http.post(url, $scope.config_text)
                .success(function (data) {
                v.subject = data.Subject;
                v.body = $sce.trustAsHtml(data.Body);
            })
                .error(function (error) {
                v.error = error;
            })
                .finally(function () {
                v.doneLoading = true;
            });
        };
        function procResults(data) {
            $scope.subject = data.Subject;
            $scope.body = $sce.trustAsHtml(data.Body);
            $scope.data = JSON.stringify(data.Data, null, '  ');
            $scope.error = data.Errors;
            $scope.warning = data.Warnings;
        }
        $scope.downloadConfig = function () {
            var blob = new Blob([$scope.config_text], { type: "text/plain;charset=utf-8" });
            saveAs(blob, "bosun.conf");
        };
        $scope.saveConfig = function () {
            $scope.saveResult = "Saving; Please Wait";
            $http.post('/api/config/save', { "Config": $scope.config_text })
                .success(function (data) {
                $scope.saveResult = "Config Saved; Reloading";
            })
                .error(function (error) {
                $scope.saveResult = error;
            });
        };
        $scope.saveClass = function () {
            if ($scope.saveResult == "Saving; Please Wait") {
                return "alert-warning";
            }
            if ($scope.saveResult == "Config Saved; Reloading") {
                return "alert-success";
            }
            return "alert-danger";
        };
        return $scope;
    }]);
bosunControllers.controller('DashboardCtrl', ['$scope', '$http', '$location', function ($scope, $http, $location) {
        var search = $location.search();
        $scope.loading = 'Loading';
        $scope.error = '';
        $scope.filter = search.filter;
        if (!$scope.filter) {
            $scope.filter = readCookie("filter");
        }
        $location.search('filter', $scope.filter || null);
        reload();
        function reload() {
            $scope.refresh($scope.filter).then(function () {
                $scope.loading = '';
                $scope.error = '';
            }, function (err) {
                $scope.loading = '';
                $scope.error = 'Unable to fetch alerts: ' + err;
            });
        }
        $scope.keydown = function ($event) {
            if ($event.keyCode == 13) {
                createCookie("filter", $scope.filter || "", 1000);
                $location.search('filter', $scope.filter || null);
            }
        };
    }]);
bosunApp.directive('tsResults', function () {
    return {
        templateUrl: '/partials/results.html',
        link: function (scope, elem, attrs) {
            scope.isSeries = function (v) {
                return typeof (v) === 'object';
            };
        }
    };
});
bosunApp.directive('tsComputations', function () {
    return {
        scope: {
            computations: '=tsComputations',
            time: '=',
            header: '='
        },
        templateUrl: '/partials/computations.html',
        link: function (scope, elem, attrs) {
            if (scope.time) {
                var m = moment.utc(scope.time);
                scope.timeParam = "&date=" + encodeURIComponent(m.format("YYYY-MM-DD")) + "&time=" + encodeURIComponent(m.format("HH:mm"));
            }
            scope.btoa = function (v) {
                return encodeURIComponent(btoa(v));
            };
        }
    };
});
function fmtDuration(v) {
    var diff = moment.duration(v, 'milliseconds');
    var f;
    if (Math.abs(v) < 60000) {
        return diff.format('ss[s]');
    }
    return diff.format('d[d]hh[h]mm[m]ss[s]');
}
function fmtTime(v) {
    var m = moment(v).utc();
    var now = moment().utc();
    var msdiff = now.diff(m);
    var ago = '';
    var inn = '';
    if (msdiff >= 0) {
        ago = ' ago';
    }
    else {
        inn = 'in ';
    }
    return m.format() + ' (' + inn + fmtDuration(msdiff) + ago + ')';
}
function parseDuration(v) {
    var pattern = /(\d+)(d|y|n|h|m|s)-ago/;
    var m = pattern.exec(v);
    return moment.duration(parseInt(m[1]), m[2].replace('n', 'M'));
}
bosunApp.directive("tsTime", function () {
    return {
        link: function (scope, elem, attrs) {
            scope.$watch(attrs.tsTime, function (v) {
                var m = moment(v).utc();
                var text = fmtTime(v);
                if (attrs.tsEndTime) {
                    var diff = moment(scope.$eval(attrs.tsEndTime)).diff(m);
                    var duration = fmtDuration(diff);
                    text += " for " + duration;
                }
                if (attrs.noLink) {
                    elem.text(text);
                }
                else {
                    var el = document.createElement('a');
                    el.text = text;
                    el.href = 'http://www.timeanddate.com/worldclock/converted.html?iso=';
                    el.href += m.format('YYYYMMDDTHHmm');
                    el.href += '&p1=0';
                    angular.forEach(scope.timeanddate, function (v, k) {
                        el.href += '&p' + (k + 2) + '=' + v;
                    });
                    elem.html(el);
                }
            });
        }
    };
});
bosunApp.directive("tsTimeUnix", function () {
    return {
        link: function (scope, elem, attrs) {
            scope.$watch(attrs.tsTimeUnix, function (v) {
                var m = moment(v * 1000).utc();
                var text = fmtTime(m);
                if (attrs.tsEndTime) {
                    var diff = moment(scope.$eval(attrs.tsEndTime)).diff(m);
                    var duration = fmtDuration(diff);
                    text += " for " + duration;
                }
                if (attrs.noLink) {
                    elem.text(text);
                }
                else {
                    var el = document.createElement('a');
                    el.text = text;
                    el.href = 'http://www.timeanddate.com/worldclock/converted.html?iso=';
                    el.href += m.format('YYYYMMDDTHHmm');
                    el.href += '&p1=0';
                    angular.forEach(scope.timeanddate, function (v, k) {
                        el.href += '&p' + (k + 2) + '=' + v;
                    });
                    elem.html(el);
                }
            });
        }
    };
});
bosunApp.directive("tsSince", function () {
    return {
        link: function (scope, elem, attrs) {
            scope.$watch(attrs.tsSince, function (v) {
                var m = moment(v).utc();
                elem.text(m.fromNow());
            });
        }
    };
});
bosunApp.directive("tooltip", function () {
    return {
        link: function (scope, elem, attrs) {
            angular.element(elem[0]).tooltip({ placement: "bottom" });
        }
    };
});
bosunApp.directive('tsTab', function () {
    return {
        link: function (scope, elem, attrs) {
            var ta = elem[0];
            elem.keydown(function (evt) {
                if (evt.ctrlKey) {
                    return;
                }
                // This is so shift-enter can be caught to run a rule when tsTab is called from
                // the rule page
                if (evt.keyCode == 13 && evt.shiftKey) {
                    return;
                }
                switch (evt.keyCode) {
                    case 9:
                        evt.preventDefault();
                        var v = ta.value;
                        var start = ta.selectionStart;
                        ta.value = v.substr(0, start) + "\t" + v.substr(start);
                        ta.selectionStart = ta.selectionEnd = start + 1;
                        return;
                    case 13:
                        if (ta.selectionStart != ta.selectionEnd) {
                            return;
                        }
                        evt.preventDefault();
                        var v = ta.value;
                        var start = ta.selectionStart;
                        var sub = v.substr(0, start);
                        var last = sub.lastIndexOf("\n") + 1;
                        for (var i = last; i < sub.length && /[ \t]/.test(sub[i]); i++)
                            ;
                        var ws = sub.substr(last, i - last);
                        ta.value = v.substr(0, start) + "\n" + ws + v.substr(start);
                        ta.selectionStart = ta.selectionEnd = start + 1 + ws.length;
                }
            });
        }
    };
});
bosunApp.directive('tsresizable', function () {
    return {
        restrict: 'A',
        scope: {
            callback: '&onResize'
        },
        link: function postLink(scope, elem, attrs) {
            elem.resizable();
            elem.on('resizestop', function (evt, ui) {
                if (scope.callback) {
                    scope.callback();
                }
            });
        }
    };
});
bosunApp.directive('tsTableSort', ['$timeout', function ($timeout) {
        return {
            link: function (scope, elem, attrs) {
                $timeout(function () {
                    $(elem).tablesorter({
                        sortList: scope.$eval(attrs.tsTableSort)
                    });
                });
            }
        };
    }]);
bosunApp.directive('tsTimeLine', function () {
    var tsdbFormat = d3.time.format.utc("%Y/%m/%d-%X");
    function parseDate(s) {
        return moment.utc(s).toDate();
    }
    var margin = {
        top: 10,
        right: 10,
        bottom: 30,
        left: 250
    };
    return {
        link: function (scope, elem, attrs) {
            scope.shown = {};
            scope.collapse = function (i, entry, v) {
                scope.shown[i] = !scope.shown[i];
                if (scope.loadTimelinePanel && entry && scope.shown[i]) {
                    scope.loadTimelinePanel(entry, v);
                }
            };
            scope.$watch('alert_history', update);
            function update(history) {
                if (!history) {
                    return;
                }
                var entries = d3.entries(history);
                if (!entries.length) {
                    return;
                }
                entries.sort(function (a, b) {
                    return a.key.localeCompare(b.key);
                });
                scope.entries = entries;
                var values = entries.map(function (v) { return v.value; });
                var keys = entries.map(function (v) { return v.key; });
                var barheight = 500 / values.length;
                barheight = Math.min(barheight, 45);
                barheight = Math.max(barheight, 15);
                var svgHeight = values.length * barheight + margin.top + margin.bottom;
                var height = svgHeight - margin.top - margin.bottom;
                var svgWidth = elem.width();
                var width = svgWidth - margin.left - margin.right;
                var xScale = d3.time.scale.utc().range([0, width]);
                var xAxis = d3.svg.axis()
                    .scale(xScale)
                    .orient('bottom');
                elem.empty();
                var svg = d3.select(elem[0])
                    .append('svg')
                    .attr('width', svgWidth)
                    .attr('height', svgHeight)
                    .append('g')
                    .attr('transform', 'translate(' + margin.left + ',' + margin.top + ')');
                svg.append('g')
                    .attr('class', 'x axis tl-axis')
                    .attr('transform', 'translate(0,' + height + ')');
                xScale.domain([
                    d3.min(values, function (d) { return d3.min(d.History, function (c) { return parseDate(c.Time); }); }),
                    d3.max(values, function (d) { return d3.max(d.History, function (c) { return parseDate(c.EndTime); }); }),
                ]);
                var legend = d3.select(elem[0])
                    .append('div')
                    .attr('class', 'tl-legend');
                var time_legend = legend
                    .append('div')
                    .text(values[0].History[0].Time);
                var alert_legend = legend
                    .append('div')
                    .text(keys[0]);
                svg.select('.x.axis')
                    .transition()
                    .call(xAxis);
                var chart = svg.append('g');
                angular.forEach(entries, function (entry, i) {
                    chart.selectAll('.bars')
                        .data(entry.value.History)
                        .enter()
                        .append('rect')
                        .attr('class', function (d) { return 'tl-' + d.Status; })
                        .attr('x', function (d) { return xScale(parseDate(d.Time)); })
                        .attr('y', i * barheight)
                        .attr('height', barheight)
                        .attr('width', function (d) {
                        return xScale(parseDate(d.EndTime)) - xScale(parseDate(d.Time));
                    })
                        .on('mousemove.x', mousemove_x)
                        .on('mousemove.y', function (d) {
                        alert_legend.text(entry.key);
                    })
                        .on('click', function (d, j) {
                        var id = 'panel' + i + '-' + j;
                        scope.shown['group' + i] = true;
                        scope.shown[id] = true;
                        if (scope.loadTimelinePanel) {
                            scope.loadTimelinePanel(entry, d);
                        }
                        scope.$apply();
                        setTimeout(function () {
                            var e = $("#" + id);
                            if (!e) {
                                console.log('no', id, e);
                                return;
                            }
                            $('html, body').scrollTop(e.offset().top);
                        });
                    });
                });
                chart.selectAll('.labels')
                    .data(keys)
                    .enter()
                    .append('text')
                    .attr('text-anchor', 'end')
                    .attr('x', 0)
                    .attr('dx', '-.5em')
                    .attr('dy', '.25em')
                    .attr('y', function (d, i) { return (i + .5) * barheight; })
                    .text(function (d) { return d; });
                chart.selectAll('.sep')
                    .data(values)
                    .enter()
                    .append('rect')
                    .attr('y', function (d, i) { return (i + 1) * barheight; })
                    .attr('height', 1)
                    .attr('x', 0)
                    .attr('width', width)
                    .on('mousemove.x', mousemove_x);
                function mousemove_x() {
                    var x = xScale.invert(d3.mouse(this)[0]);
                    time_legend
                        .text(tsdbFormat(x));
                }
            }
            ;
        }
    };
});
var fmtUnits = ['', 'k', 'M', 'G', 'T', 'P', 'E'];
function nfmt(s, mult, suffix, opts) {
    opts = opts || {};
    var n = parseFloat(s);
    if (isNaN(n) && typeof s === 'string') {
        return s;
    }
    if (opts.round)
        n = Math.round(n);
    if (!n)
        return suffix ? '0 ' + suffix : '0';
    if (isNaN(n) || !isFinite(n))
        return '-';
    var a = Math.abs(n);
    if (a >= 1) {
        var number = Math.floor(Math.log(a) / Math.log(mult));
        a /= Math.pow(mult, Math.floor(number));
        if (fmtUnits[number]) {
            suffix = fmtUnits[number] + suffix;
        }
    }
    var r = a.toFixed(5);
    if (a < 1e-5) {
        r = a.toString();
    }
    var neg = n < 0 ? '-' : '';
    return neg + (+r) + suffix;
}
bosunApp.filter('nfmt', function () {
    return function (s) {
        return nfmt(s, 1000, '', {});
    };
});
bosunApp.filter('bytes', function () {
    return function (s) {
        return nfmt(s, 1024, 'B', { round: true });
    };
});
bosunApp.filter('bits', function () {
    return function (s) {
        return nfmt(s, 1024, 'b', { round: true });
    };
});
bosunApp.directive('elastic', [
    '$timeout',
    function ($timeout) {
        return {
            restrict: 'A',
            link: function ($scope, element) {
                $scope.initialHeight = $scope.initialHeight || element[0].style.height;
                var resize = function () {
                    element[0].style.height = $scope.initialHeight;
                    element[0].style.height = "" + element[0].scrollHeight + "px";
                };
                element.on("input change", resize);
                $timeout(resize, 0);
            }
        };
    }
]);
bosunApp.directive('tsBar', ['$window', 'nfmtFilter', function ($window, fmtfilter) {
        var margin = {
            top: 20,
            right: 20,
            bottom: 0,
            left: 200
        };
        return {
            scope: {
                data: '=',
                height: '='
            },
            link: function (scope, elem, attrs) {
                var svgHeight = +scope.height || 150;
                var height = svgHeight - margin.top - margin.bottom;
                var svgWidth;
                var width;
                var xScale = d3.scale.linear();
                var yScale = d3.scale.ordinal();
                var top = d3.select(elem[0])
                    .append('svg')
                    .attr('height', svgHeight)
                    .attr('width', '100%');
                var svg = top
                    .append('g');
                //.attr('transform', 'translate(' + margin.left + ',' + margin.top + ')');
                var xAxis = d3.svg.axis()
                    .scale(xScale)
                    .orient("top");
                var yAxis = d3.svg.axis()
                    .scale(yScale)
                    .orient("left");
                scope.$watch('data', update);
                var w = angular.element($window);
                scope.$watch(function () {
                    return w.width();
                }, resize, true);
                w.bind('resize', function () {
                    scope.$apply();
                });
                function resize() {
                    if (!scope.data) {
                        return;
                    }
                    svgWidth = elem.width();
                    if (svgWidth <= 0) {
                        return;
                    }
                    margin.left = d3.max(scope.data, function (d) { return d.name.length * 8; });
                    width = svgWidth - margin.left - margin.right;
                    svgHeight = scope.data.length * 15;
                    height = svgHeight - margin.top - margin.bottom;
                    xScale.range([0, width]);
                    yScale.rangeRoundBands([0, height], .1);
                    yAxis.scale(yScale);
                    svg.attr('transform', 'translate(' + margin.left + ',' + margin.top + ')');
                    svg.attr('width', svgWidth);
                    svg.attr('height', height);
                    top.attr('height', svgHeight);
                    xAxis.ticks(width / 60);
                    draw();
                }
                function update(v) {
                    if (!angular.isArray(v) || v.length == 0) {
                        return;
                    }
                    resize();
                }
                function draw() {
                    if (!scope.data) {
                        return;
                    }
                    yScale.domain(scope.data.map(function (d) { return d.name; }));
                    xScale.domain([0, d3.max(scope.data, function (d) { return d.Value; })]);
                    svg.selectAll('g.axis').remove();
                    //X axis
                    svg.append("g")
                        .attr("class", "x axis")
                        .call(xAxis);
                    svg.append("g")
                        .attr("class", "y axis")
                        .call(yAxis)
                        .selectAll("text")
                        .style("text-anchor", "end");
                    var bars = svg.selectAll(".bar").data(scope.data);
                    bars.enter()
                        .append("rect")
                        .attr("class", "bar")
                        .attr("y", function (d) { return yScale(d.name); })
                        .attr("height", yScale.rangeBand())
                        .attr('width', function (d) { return xScale(d.Value); });
                }
                ;
            }
        };
    }]);
bosunApp.directive('tsGraph', ['$window', 'nfmtFilter', function ($window, fmtfilter) {
        var margin = {
            top: 10,
            right: 10,
            bottom: 30,
            left: 80
        };
        return {
            scope: {
                data: '=',
                annotations: '=',
                height: '=',
                generator: '=',
                brushStart: '=bstart',
                brushEnd: '=bend',
                enableBrush: '@',
                max: '=',
                min: '=',
                normalize: '=',
                annotation: '=',
                annotateEnabled: '=',
                showAnnotations: '='
            },
            template: '<div class="row"></div>' +
                '<div class="row col-lg-12"></div>' +
                '<div class"row">' +
                '<div class="col-lg-6"></div>' +
                '<div class="col-lg-6"></div>' +
                '</div>',
            link: function (scope, elem, attrs, $compile) {
                var chartElem = d3.select(elem.children()[0]);
                var timeElem = d3.select(elem.children()[1]);
                var legendAnnContainer = angular.element(elem.children()[2]);
                var legendElem = d3.select(legendAnnContainer.children()[0]);
                if (scope.annotateEnabled) {
                    var annElem = d3.select(legendAnnContainer.children()[1]);
                }
                var valueIdx = 1;
                if (scope.normalize) {
                    valueIdx = 2;
                }
                var svgHeight = +scope.height || 150;
                var height = svgHeight - margin.top - margin.bottom;
                var svgWidth;
                var width;
                var yScale = d3.scale.linear().range([height, 0]);
                var xScale = d3.time.scale.utc();
                var xAxis = d3.svg.axis()
                    .orient('bottom');
                var yAxis = d3.svg.axis()
                    .scale(yScale)
                    .orient('left')
                    .ticks(Math.min(10, height / 20))
                    .tickFormat(fmtfilter);
                var line;
                switch (scope.generator) {
                    case 'area':
                        line = d3.svg.area();
                        break;
                    default:
                        line = d3.svg.line();
                }
                var brush = d3.svg.brush()
                    .x(xScale)
                    .on('brush', brushed)
                    .on('brushend', annotateBrushed);
                var top = chartElem
                    .append('svg')
                    .attr('height', svgHeight)
                    .attr('width', '100%');
                var svg = top
                    .append('g')
                    .attr('transform', 'translate(' + margin.left + ',' + margin.top + ')');
                var defs = svg.append('defs')
                    .append('clipPath')
                    .attr('id', 'clip')
                    .append('rect')
                    .attr('height', height);
                var chart = svg.append('g')
                    .attr('pointer-events', 'all')
                    .attr('clip-path', 'url(#clip)');
                svg.append('g')
                    .attr('class', 'x axis')
                    .attr('transform', 'translate(0,' + height + ')');
                svg.append('g')
                    .attr('class', 'y axis');
                var paths = chart.append('g');
                chart.append('g')
                    .attr('class', 'x brush');
                if (scope.annotateEnabled) {
                    var ann = chart.append('g');
                }
                top.append('rect')
                    .style('opacity', 0)
                    .attr('x', 0)
                    .attr('y', 0)
                    .attr('height', height)
                    .attr('width', margin.left)
                    .style('cursor', 'pointer')
                    .on('click', yaxisToggle);
                var xloc = timeElem.append('div').attr("class", "col-lg-6");
                xloc.style('float', 'left');
                var brushText = timeElem.append('div').attr("class", "col-lg-6").append('p').attr("class", "text-right");
                var legend = legendElem;
                var aLegend = annElem;
                var color = d3.scale.ordinal().range([
                    '#e41a1c',
                    '#377eb8',
                    '#4daf4a',
                    '#984ea3',
                    '#ff7f00',
                    '#a65628',
                    '#f781bf',
                    '#999999',
                ]);
                var annColor = d3.scale.ordinal().range([
                    '#e41a1c',
                    '#377eb8',
                    '#4daf4a',
                    '#984ea3',
                    '#ff7f00',
                    '#a65628',
                    '#f781bf',
                    '#999999',
                ]);
                var mousex = 0;
                var mousey = 0;
                var oldx = 0;
                var hover = svg.append('g')
                    .attr('class', 'hover')
                    .style('pointer-events', 'none')
                    .style('display', 'none');
                var hoverPoint = hover.append('svg:circle')
                    .attr('r', 5);
                var hoverRect = hover.append('svg:rect')
                    .attr('fill', 'white');
                var hoverText = hover.append('svg:text')
                    .style('font-size', '12px');
                var focus = svg.append('g')
                    .attr('class', 'focus')
                    .style('pointer-events', 'none');
                focus.append('line');
                var yaxisZero = false;
                function yaxisToggle() {
                    yaxisZero = !yaxisZero;
                    draw();
                }
                var drawAnnLegend = function () {
                    if (scope.annotation) {
                        aLegend.html('');
                        var a = scope.annotation;
                        //var table = aLegend.append('table').attr("class", "table table-condensed")
                        var table = aLegend.append("div");
                        var row = table.append("div").attr("class", "row");
                        row.append("div").attr("class", "col-lg-2").text("CreationUser");
                        row.append("div").attr("class", "col-lg-10").text(a.CreationUser);
                        row = table.append("div").attr("class", "row");
                        row.append("div").attr("class", "col-lg-2").text("Owner");
                        row.append("div").attr("class", "col-lg-10").text(a.Owner);
                        row = table.append("div").attr("class", "row");
                        row.append("div").attr("class", "col-lg-2").text("Url");
                        row.append("div").attr("class", "col-lg-10").append('a')
                            .attr("xlink:href", a.Url).text(a.Url).on("click", function (d) {
                            window.open(a.Url, "_blank");
                        });
                        row = table.append("div").attr("class", "row");
                        row.append("div").attr("class", "col-lg-2").text("Category");
                        row.append("div").attr("class", "col-lg-10").text(a.Category);
                        row = table.append("div").attr("class", "row");
                        row.append("div").attr("class", "col-lg-2").text("Host");
                        row.append("div").attr("class", "col-lg-10").text(a.Host);
                        row = table.append("div").attr("class", "row");
                        row.append("div").attr("class", "col-lg-2").text("Message");
                        row.append("div").attr("class", "col-lg-10").text(a.Message);
                    } //
                };
                var drawLegend = _.throttle(function (normalizeIdx) {
                    var names = legend.selectAll('.series')
                        .data(scope.data, function (d) { return d.Name; });
                    names.enter()
                        .append('div')
                        .attr('class', 'series');
                    names.exit()
                        .remove();
                    var xi = xScale.invert(mousex);
                    xloc.text('Time: ' + fmtTime(xi));
                    var t = xi.getTime() / 1000;
                    var minDist = width + height;
                    var minName, minColor;
                    var minX, minY;
                    names
                        .each(function (d) {
                        var idx = bisect(d.Data, t);
                        if (idx >= d.Data.length) {
                            idx = d.Data.length - 1;
                        }
                        var e = d3.select(this);
                        var pt = d.Data[idx];
                        if (pt) {
                            e.attr('title', pt[normalizeIdx]);
                            e.text(d.Name + ': ' + fmtfilter(pt[1]));
                            var ptx = xScale(pt[0] * 1000);
                            var pty = yScale(pt[normalizeIdx]);
                            var ptd = Math.sqrt(Math.pow(ptx - mousex, 2) +
                                Math.pow(pty - mousey, 2));
                            if (ptd < minDist) {
                                minDist = ptd;
                                minX = ptx;
                                minY = pty;
                                minName = d.Name + ': ' + pt[1];
                                minColor = color(d.Name);
                            }
                        }
                    })
                        .style('color', function (d) { return color(d.Name); });
                    hover
                        .attr('transform', 'translate(' + minX + ',' + minY + ')');
                    hoverPoint.style('fill', minColor);
                    hoverText
                        .text(minName)
                        .style('fill', minColor);
                    var isRight = minX > width / 2;
                    var isBottom = minY > height / 2;
                    hoverText
                        .attr('x', isRight ? -5 : 5)
                        .attr('y', isBottom ? -8 : 15)
                        .attr('text-anchor', isRight ? 'end' : 'start');
                    var node = hoverText.node();
                    var bb = node.getBBox();
                    hoverRect
                        .attr('x', bb.x - 1)
                        .attr('y', bb.y - 1)
                        .attr('height', bb.height + 2)
                        .attr('width', bb.width + 2);
                    var x = mousex;
                    if (x > width) {
                        x = 0;
                    }
                    focus.select('line')
                        .attr('x1', x)
                        .attr('x2', x)
                        .attr('y1', 0)
                        .attr('y2', height);
                    if (extentStart) {
                        var s = extentStart;
                        if (extentEnd != extentStart) {
                            s += ' - ' + extentEnd;
                            s += ' (' + extentDiff + ')';
                        }
                        brushText.text(s);
                    }
                }, 50);
                scope.$watchCollection('[data, annotations, showAnnotations]', update);
                var showAnnotations = function (show) {
                    if (show) {
                        ann.attr("visibility", "visible");
                        return;
                    }
                    ann.attr("visibility", "hidden");
                    aLegend.html('');
                };
                var w = angular.element($window);
                scope.$watch(function () {
                    return w.width();
                }, resize, true);
                w.bind('resize', function () {
                    scope.$apply();
                });
                function resize() {
                    svgWidth = elem.width();
                    if (svgWidth <= 0) {
                        return;
                    }
                    width = svgWidth - margin.left - margin.right;
                    xScale.range([0, width]);
                    xAxis.scale(xScale);
                    if (!mousex) {
                        mousex = width + 1;
                    }
                    svg.attr('width', svgWidth);
                    defs.attr('width', width);
                    xAxis.ticks(width / 60);
                    draw();
                }
                var oldx = 0;
                var bisect = d3.bisector(function (d) { return d[0]; }).left;
                var bisectA = d3.bisector(function (d) { return moment(d.StartDate).unix(); }).left;
                function update(v) {
                    if (!angular.isArray(v) || v.length == 0) {
                        return;
                    }
                    d3.selectAll(".x.brush").call(brush.clear());
                    if (scope.annotateEnabled) {
                        showAnnotations(scope.showAnnotations);
                    }
                    resize();
                }
                function draw() {
                    if (!scope.data) {
                        return;
                    }
                    if (scope.normalize) {
                        valueIdx = 2;
                    }
                    function mousemove() {
                        var pt = d3.mouse(this);
                        mousex = pt[0];
                        mousey = pt[1];
                        drawLegend(valueIdx);
                    }
                    scope.data.map(function (data, i) {
                        var max = d3.max(data.Data, function (d) { return d[1]; });
                        data.Data.map(function (d, j) {
                            d.push(d[1] / max * 100 || 0);
                        });
                    });
                    line.y(function (d) { return yScale(d[valueIdx]); });
                    line.x(function (d) { return xScale(d[0] * 1000); });
                    var xdomain = [
                        d3.min(scope.data, function (d) { return d3.min(d.Data, function (c) { return c[0]; }); }) * 1000,
                        d3.max(scope.data, function (d) { return d3.max(d.Data, function (c) { return c[0]; }); }) * 1000,
                    ];
                    if (!oldx) {
                        oldx = xdomain[1];
                    }
                    xScale.domain(xdomain);
                    var ymin = d3.min(scope.data, function (d) { return d3.min(d.Data, function (c) { return c[1]; }); });
                    var ymax = d3.max(scope.data, function (d) { return d3.max(d.Data, function (c) { return c[valueIdx]; }); });
                    var diff = (ymax - ymin) / 50;
                    if (!diff) {
                        diff = 1;
                    }
                    ymin -= diff;
                    ymax += diff;
                    if (yaxisZero) {
                        if (ymin > 0) {
                            ymin = 0;
                        }
                        else if (ymax < 0) {
                            ymax = 0;
                        }
                    }
                    var ydomain = [ymin, ymax];
                    if (angular.isNumber(scope.min)) {
                        ydomain[0] = +scope.min;
                    }
                    if (angular.isNumber(scope.max)) {
                        ydomain[valueIdx] = +scope.max;
                    }
                    yScale.domain(ydomain);
                    if (scope.generator == 'area') {
                        line.y0(yScale(0));
                    }
                    svg.select('.x.axis')
                        .transition()
                        .call(xAxis);
                    svg.select('.y.axis')
                        .transition()
                        .call(yAxis);
                    svg.append('text')
                        .attr("class", "ylabel")
                        .attr("transform", "rotate(-90)")
                        .attr("y", -margin.left)
                        .attr("x", -(height / 2))
                        .attr("dy", "1em")
                        .text(_.uniq(scope.data.map(function (v) { return v.Unit; })).join("; "));
                    if (scope.annotateEnabled) {
                        var rowId = {}; // annotation Id -> rowId
                        var rowEndDate = {}; // rowId -> EndDate
                        var maxRow = 0;
                        for (var i = 0; i < scope.annotations.length; i++) {
                            if (i == 0) {
                                rowId[scope.annotations[i].Id] = 0;
                                rowEndDate[0] = scope.annotations[0].EndDate;
                                continue;
                            }
                            for (var row = 0; row <= maxRow + 1; row++) {
                                if (row == maxRow + 1) {
                                    rowId[scope.annotations[i].Id] = row;
                                    rowEndDate[row] = scope.annotations[i].EndDate;
                                    maxRow += 1;
                                    break;
                                }
                                if (rowEndDate[row] < scope.annotations[i].StartDate) {
                                    rowId[scope.annotations[i].Id] = row;
                                    rowEndDate[row] = scope.annotations[i].EndDate;
                                    break;
                                }
                            }
                        }
                        var annotations = ann.selectAll('.annotation')
                            .data(scope.annotations, function (d) { return d.Id; });
                        annotations.enter()
                            .append("svg:a")
                            .append('rect')
                            .attr('visilibity', function () {
                            if (scope.showAnnotations) {
                                return "visible";
                            }
                            return "hidden";
                        })
                            .attr("y", function (d) { return rowId[d.Id] * ((height * .05) + 2); })
                            .attr("height", height * .05)
                            .attr("class", "annotation")
                            .attr("stroke", function (d) { return annColor(d.Id); })
                            .attr("stroke-opacity", .5)
                            .attr("fill", function (d) { return annColor(d.Id); })
                            .attr("fill-opacity", 0.1)
                            .attr("stroke-width", 1)
                            .attr("x", function (d) { return xScale(moment(d.StartDate).utc().unix() * 1000); })
                            .attr("width", function (d) {
                            var startT = moment(d.StartDate).utc().unix() * 1000;
                            var endT = moment(d.EndDate).utc().unix() * 1000;
                            if (startT == endT) {
                                return 3;
                            }
                            return xScale(endT) - xScale(startT);
                        })
                            .on("mouseenter", function (ann) {
                            if (!scope.showAnnotations) {
                                return;
                            }
                            if (ann) {
                                scope.annotation = ann;
                                drawAnnLegend();
                            }
                            scope.$apply();
                        })
                            .on("click", function () {
                            if (!scope.showAnnotations) {
                                return;
                            }
                            angular.element('#modalShower').trigger('click');
                        });
                        annotations.exit().remove();
                    }
                    var queries = paths.selectAll('.line')
                        .data(scope.data, function (d) { return d.Name; });
                    switch (scope.generator) {
                        case 'area':
                            queries.enter()
                                .append('path')
                                .attr('stroke', function (d) { return color(d.Name); })
                                .attr('class', 'line')
                                .style('fill', function (d) { return color(d.Name); });
                            break;
                        default:
                            queries.enter()
                                .append('path')
                                .attr('stroke', function (d) { return color(d.Name); })
                                .attr('class', 'line');
                    }
                    queries.exit()
                        .remove();
                    queries
                        .attr('d', function (d) { return line(d.Data); })
                        .attr('transform', null)
                        .transition()
                        .ease('linear')
                        .attr('transform', 'translate(' + (xScale(oldx) - xScale(xdomain[1])) + ')');
                    chart.select('.x.brush')
                        .call(brush)
                        .selectAll('rect')
                        .attr('height', height)
                        .on('mouseover', function () {
                        hover.style('display', 'block');
                    })
                        .on('mouseout', function () {
                        hover.style('display', 'none');
                    })
                        .on('mousemove', mousemove);
                    chart.select('.x.brush .extent')
                        .style('stroke', '#fff')
                        .style('fill-opacity', '.125')
                        .style('shape-rendering', 'crispEdges');
                    oldx = xdomain[1];
                    drawLegend(valueIdx);
                }
                ;
                var extentStart;
                var extentEnd;
                var extentDiff;
                function brushed() {
                    var e;
                    e = d3.event.sourceEvent;
                    if (e.shiftKey) {
                        return;
                    }
                    var extent = brush.extent();
                    extentStart = datefmt(extent[0]);
                    extentEnd = datefmt(extent[1]);
                    extentDiff = fmtDuration(moment(extent[1]).diff(moment(extent[0])));
                    drawLegend(valueIdx);
                    if (scope.enableBrush && extentEnd != extentStart) {
                        scope.brushStart = extentStart;
                        scope.brushEnd = extentEnd;
                        scope.$apply();
                    }
                }
                function annotateBrushed() {
                    if (!scope.annotateEnabled) {
                        return;
                    }
                    var e;
                    e = d3.event.sourceEvent;
                    if (!e.shiftKey) {
                        return;
                    }
                    var extent = brush.extent();
                    scope.annotation = new Annotation();
                    scope.annotation.StartDate = moment(extent[0]).utc().format(timeFormat);
                    scope.annotation.EndDate = moment(extent[1]).utc().format(timeFormat);
                    scope.$apply(); // This logs a console type error, but also works .. odd.
                    angular.element('#modalShower').trigger('click');
                }
                var mfmt = 'YYYY/MM/DD-HH:mm:ss';
                function datefmt(d) {
                    return moment(d).utc().format(mfmt);
                }
            }
        };
    }]);
bosunControllers.controller('ErrorCtrl', ['$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        $scope.loading = true;
        $http.get('/api/errors')
            .success(function (data) {
            $scope.errors = [];
            _(data).forEach(function (err, name) {
                err.Name = name;
                err.Sum = 0;
                err.Shown = true;
                _(err.Errors).forEach(function (line) {
                    err.Sum += line.Count;
                    line.FirstTime = moment.utc(line.FirstTime);
                    line.LastTime = moment.utc(line.LastTime);
                });
                $scope.errors.push(err);
            });
        })
            .error(function (data) {
            $scope.error = "Error fetching data: " + data;
        })
            .finally(function () { $scope.loading = false; });
        $scope.click = function (err, event) {
            event.stopPropagation();
        };
        $scope.totalLines = function () {
            return $scope.errors.length;
        };
        $scope.selectedLines = function () {
            var t = 0;
            _($scope.errors).forEach(function (err) {
                if (err.checked) {
                    t++;
                }
            });
            return t;
        };
        var getChecked = function () {
            var keys = [];
            _($scope.errors).forEach(function (err) {
                if (err.checked) {
                    keys.push(err.Name);
                }
            });
            return keys;
        };
        var clear = function (keys) {
            $http.post('/api/errors', keys)
                .success(function (data) {
                $route.reload();
            })
                .error(function (data) {
                $scope.error = "Error Clearing Errors: " + data;
            });
        };
        $scope.clearAll = function () {
            clear(["all"]);
        };
        $scope.clearSelected = function () {
            var keys = getChecked();
            clear(keys);
        };
        $scope.ruleLink = function (line, err) {
            var url = "/config?alert=" + err.Name;
            var fromDate = moment.utc(line.FirstTime);
            url += "&fromDate=" + fromDate.format("YYYY-MM-DD");
            url += "&fromTime=" + fromDate.format("hh:mm");
            var toDate = moment.utc(line.LastTime);
            url += "&toDate=" + toDate.format("YYYY-MM-DD");
            url += "&toTime=" + toDate.format("hh:mm");
            return url;
        };
    }]);
bosunControllers.controller('ExprCtrl', ['$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        var search = $location.search();
        var current;
        try {
            current = atob(search.expr);
        }
        catch (e) {
            current = '';
        }
        if (!current) {
            $location.search('expr', btoa('avg(q("avg:rate:os.cpu{host=*bosun*}", "5m", "")) > 80'));
            return;
        }
        $scope.date = search.date || '';
        $scope.time = search.time || '';
        $scope.expr = current;
        $scope.running = current;
        $scope.tab = search.tab || 'results';
        $scope.animate();
        $http.post('/api/expr?' +
            'date=' + encodeURIComponent($scope.date) +
            '&time=' + encodeURIComponent($scope.time), current)
            .success(function (data) {
            $scope.result = data.Results;
            $scope.queries = data.Queries;
            $scope.result_type = data.Type;
            if (data.Type == 'series') {
                $scope.svg_url = '/api/egraph/' + btoa(current) + '.svg?now=' + Math.floor(Date.now() / 1000);
                $scope.graph = toChart(data.Results);
            }
            if (data.Type == 'number') {
                angular.forEach(data.Results, function (d) {
                    var name = '{';
                    angular.forEach(d.Group, function (tagv, tagk) {
                        if (name.length > 1) {
                            name += ',';
                        }
                        name += tagk + '=' + tagv;
                    });
                    name += '}';
                    d.name = name;
                });
                $scope.bar = data.Results;
            }
            $scope.running = '';
        })
            .error(function (error) {
            $scope.error = error;
            $scope.running = '';
        })
            .finally(function () {
            $scope.stop();
        });
        $scope.set = function () {
            $location.search('expr', btoa($scope.expr));
            $location.search('date', $scope.date || null);
            $location.search('time', $scope.time || null);
            $route.reload();
        };
        function toChart(res) {
            var graph = [];
            angular.forEach(res, function (d, idx) {
                var data = [];
                angular.forEach(d.Value, function (val, ts) {
                    data.push([+ts, val]);
                });
                if (data.length == 0) {
                    return;
                }
                var name = '{';
                angular.forEach(d.Group, function (tagv, tagk) {
                    if (name.length > 1) {
                        name += ',';
                    }
                    name += tagk + '=' + tagv;
                });
                name += '}';
                var series = {
                    Data: data,
                    Name: name
                };
                graph[idx] = series;
            });
            return graph;
        }
        $scope.keydown = function ($event) {
            if ($event.shiftKey && $event.keyCode == 13) {
                $scope.set();
            }
        };
    }]);
var TagSet = (function () {
    function TagSet() {
    }
    return TagSet;
})();
var TagV = (function () {
    function TagV() {
    }
    return TagV;
})();
var RateOptions = (function () {
    function RateOptions() {
    }
    return RateOptions;
})();
var Filter = (function () {
    function Filter(f) {
        this.type = f && f.type || "auto";
        this.tagk = f && f.tagk || "";
        this.filter = f && f.filter || "";
        this.groupBy = f && f.groupBy || false;
    }
    return Filter;
})();
var FilterMap = (function () {
    function FilterMap() {
    }
    return FilterMap;
})();
var Query = (function () {
    function Query(filterSupport, q) {
        this.aggregator = q && q.aggregator || 'sum';
        this.metric = q && q.metric || '';
        this.rate = q && q.rate || false;
        this.rateOptions = q && q.rateOptions || new RateOptions;
        if (q && !q.derivative) {
            // back compute derivative from q
            if (!this.rate) {
                this.derivative = 'gauge';
            }
            else if (this.rateOptions.counter) {
                this.derivative = 'counter';
            }
            else {
                this.derivative = 'rate';
            }
        }
        else {
            this.derivative = q && q.derivative || 'auto';
        }
        this.ds = q && q.ds || '';
        this.dstime = q && q.dstime || '';
        this.tags = q && q.tags || new TagSet;
        this.gbFilters = q && q.gbFilters || new FilterMap;
        this.nGbFilters = q && q.nGbFilters || new FilterMap;
        var that = this;
        // Copy tags with values to group by filters so old links work
        if (filterSupport) {
            _.each(this.tags, function (v, k) {
                if (v === "") {
                    return;
                }
                var f = new (Filter);
                f.filter = v;
                f.groupBy = true;
                f.tagk = k;
                that.gbFilters[k] = f;
            });
            // Load filters from raw query and turn them into gb and nGbFilters.
            // This makes links from other pages work (i.e. the expr page)
            if (_.has(q, 'filters')) {
                _.each(q.filters, function (filter) {
                    if (filter.groupBy) {
                        that.gbFilters[filter.tagk] = filter;
                        return;
                    }
                    that.nGbFilters[filter.tagk] = filter;
                });
            }
        }
        this.setFilters();
        this.setDs();
        this.setDerivative();
    }
    Query.prototype.setFilters = function () {
        this.filters = [];
        var that = this;
        _.each(this.gbFilters, function (filter, tagk) {
            if (filter.filter && filter.type) {
                that.filters.push(filter);
            }
        });
        _.each(this.nGbFilters, function (filter, tagk) {
            if (filter.filter && filter.type) {
                that.filters.push(filter);
            }
        });
    };
    Query.prototype.setDs = function () {
        if (this.dstime && this.ds) {
            this.downsample = this.dstime + '-' + this.ds;
        }
        else {
            this.downsample = '';
        }
    };
    Query.prototype.setDerivative = function () {
        var max = this.rateOptions.counterMax;
        this.rate = false;
        this.rateOptions = new RateOptions();
        switch (this.derivative) {
            case "rate":
                this.rate = true;
                break;
            case "counter":
                this.rate = true;
                this.rateOptions.counter = true;
                this.rateOptions.counterMax = max;
                this.rateOptions.resetValue = 1;
                break;
            case "gauge":
                this.rate = false;
                break;
        }
    };
    return Query;
})();
var Request = (function () {
    function Request() {
        this.start = '1h-ago';
        this.queries = [];
    }
    Request.prototype.prune = function () {
        var _this = this;
        for (var i = 0; i < this.queries.length; i++) {
            angular.forEach(this.queries[i], function (v, k) {
                var qi = _this.queries[i];
                switch (typeof v) {
                    case "string":
                        if (!v) {
                            delete qi[k];
                        }
                        break;
                    case "boolean":
                        if (!v) {
                            delete qi[k];
                        }
                        break;
                    case "object":
                        if (Object.keys(v).length == 0) {
                            delete qi[k];
                        }
                        break;
                }
            });
        }
    };
    return Request;
})();
var graphRefresh;
var Version = (function () {
    function Version() {
    }
    return Version;
})();
bosunControllers.controller('GraphCtrl', ['$scope', '$http', '$location', '$route', '$timeout', function ($scope, $http, $location, $route, $timeout) {
        $scope.aggregators = ["sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
        $scope.dsaggregators = ["", "sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
        $scope.filters = ["auto", "iliteral_or", "iwildcard", "literal_or", "not_iliteral_or", "not_literal_or", "regexp", "wildcard"];
        if ($scope.version.Major >= 2 && $scope.version.Minor >= 2) {
            $scope.filterSupport = true;
        }
        $scope.rate_options = ["auto", "gauge", "counter", "rate"];
        $scope.canAuto = {};
        $scope.showAnnotations = (getShowAnnotations() == "true");
        $scope.setShowAnnotations = function () {
            if ($scope.showAnnotations) {
                setShowAnnotations("true");
                return;
            }
            setShowAnnotations("false");
        };
        var search = $location.search();
        var j = search.json;
        if (search.b64) {
            j = atob(search.b64);
        }
        $scope.annotation = {};
        var request = j ? JSON.parse(j) : new Request;
        $scope.index = parseInt($location.hash()) || 0;
        $scope.tagvs = [];
        $scope.sorted_tagks = [];
        $scope.query_p = [];
        angular.forEach(request.queries, function (q, i) {
            $scope.query_p[i] = new Query($scope.filterSupport, q);
        });
        $scope.start = request.start;
        $scope.end = request.end;
        $scope.autods = search.autods != 'false';
        $scope.refresh = search.refresh == 'true';
        $scope.normalize = search.normalize == 'true';
        if (search.min) {
            $scope.min = +search.min;
        }
        if (search.max) {
            $scope.max = +search.max;
        }
        var duration_map = {
            "s": "s",
            "m": "m",
            "h": "h",
            "d": "d",
            "w": "w",
            "n": "M",
            "y": "y"
        };
        var isRel = /^(\d+)(\w)-ago$/;
        function RelToAbs(m) {
            return moment().utc().subtract(parseFloat(m[1]), duration_map[m[2]]).format();
        }
        function AbsToRel(s) {
            //Not strict parsing of the time format. For example, just "2014" will be valid
            var t = moment.utc(s, moment.defaultFormat).fromNow();
            return t;
        }
        function SwapTime(s) {
            if (!s) {
                return moment().utc().format();
            }
            var m = isRel.exec(s);
            if (m) {
                return RelToAbs(m);
            }
            return AbsToRel(s);
        }
        $scope.submitAnnotation = function () { return $http.post('/api/annotation', $scope.annotation)
            .success(function (data) {
            //debugger;
            if ($scope.annotation.Id == "" && $scope.annotation.Owner != "") {
                setOwner($scope.annotation.Owner);
            }
            $scope.annotation = new Annotation(data);
            $scope.error = "";
            // This seems to make angular refresh, where a push doesn't
            $scope.annotations = $scope.annotations.concat($scope.annotation);
        })
            .error(function (error) {
            $scope.error = error;
        }); };
        $scope.deleteAnnotation = function () { return $http.delete('/api/annotation/' + $scope.annotation.Id)
            .success(function (data) {
            $scope.error = "";
            $scope.annotations = _.without($scope.annotations, _.findWhere($scope.annotations, { Id: $scope.annotation.Id }));
        })
            .error(function (error) {
            $scope.error = error;
        }); };
        $scope.SwitchTimes = function () {
            $scope.start = SwapTime($scope.start);
            $scope.end = SwapTime($scope.end);
        };
        $scope.AddTab = function () {
            $scope.index = $scope.query_p.length;
            $scope.query_p.push(new Query($scope.filterSupport));
        };
        $scope.setIndex = function (i) {
            $scope.index = i;
        };
        var alphabet = "abcdefghijklmnopqrstuvwxyz".split("");
        if ($scope.annotateEnabled) {
            $http.get('/api/annotation/values/Owner')
                .success(function (data) {
                $scope.owners = data;
            });
            $http.get('/api/annotation/values/Category')
                .success(function (data) {
                $scope.categories = data;
            });
            $http.get('/api/annotation/values/Host')
                .success(function (data) {
                $scope.hosts = data;
            });
        }
        $scope.GetTagKByMetric = function (index) {
            $scope.tagvs[index] = new TagV;
            var metric = $scope.query_p[index].metric;
            if (!metric) {
                $scope.canAuto[metric] = true;
                return;
            }
            $http.get('/api/tagk/' + metric)
                .success(function (data) {
                var q = $scope.query_p[index];
                var tags = new TagSet;
                q.metric_tags = {};
                if (!q.gbFilters) {
                    q.gbFilters = new FilterMap;
                }
                if (!q.nGbFilters) {
                    q.nGbFilters = new FilterMap;
                }
                for (var i = 0; i < data.length; i++) {
                    var d = data[i];
                    if ($scope.filterSupport) {
                        if (!q.gbFilters[d]) {
                            var filter = new Filter;
                            filter.tagk = d;
                            filter.groupBy = true;
                            q.gbFilters[d] = filter;
                        }
                        if (!q.nGbFilters[d]) {
                            var filter = new Filter;
                            filter.tagk = d;
                            q.nGbFilters[d] = filter;
                        }
                    }
                    if (q.tags) {
                        tags[d] = q.tags[d];
                    }
                    if (!tags[d]) {
                        tags[d] = '';
                    }
                    q.metric_tags[d] = true;
                    GetTagVs(d, index);
                }
                angular.forEach(q.tags, function (val, key) {
                    if (val) {
                        tags[key] = val;
                    }
                });
                q.tags = tags;
                // Make sure host is always the first tag.
                $scope.sorted_tagks[index] = Object.keys(tags);
                $scope.sorted_tagks[index].sort(function (a, b) {
                    if (a == 'host') {
                        return -1;
                    }
                    else if (b == 'host') {
                        return 1;
                    }
                    return a.localeCompare(b);
                });
            })
                .error(function (error) {
                $scope.error = 'Unable to fetch metrics: ' + error;
            });
            $http.get('/api/metadata/metrics?metric=' + metric)
                .success(function (data) {
                var canAuto = data && data.Rate;
                $scope.canAuto[metric] = canAuto;
            })
                .error(function (err) {
                $scope.error = err;
            });
        };
        if ($scope.query_p.length == 0) {
            $scope.AddTab();
        }
        $http.get('/api/metric' + "?since=" + moment().utc().subtract(2, "days").unix())
            .success(function (data) {
            $scope.metrics = data;
        })
            .error(function (error) {
            $scope.error = 'Unable to fetch metrics: ' + error;
        });
        function GetTagVs(k, index) {
            $http.get('/api/tagv/' + k + '/' + $scope.query_p[index].metric)
                .success(function (data) {
                data.sort();
                $scope.tagvs[index][k] = data;
            })
                .error(function (error) {
                $scope.error = 'Unable to fetch metrics: ' + error;
            });
        }
        function getRequest() {
            request = new Request;
            request.start = $scope.start;
            request.end = $scope.end;
            angular.forEach($scope.query_p, function (p) {
                if (!p.metric) {
                    return;
                }
                var q = new Query($scope.filterSupport, p);
                var tags = q.tags;
                q.tags = new TagSet;
                if (!$scope.filterSupport) {
                    angular.forEach(tags, function (v, k) {
                        if (v && k) {
                            q.tags[k] = v;
                        }
                    });
                }
                request.queries.push(q);
            });
            return request;
        }
        $scope.Query = function () {
            var r = getRequest();
            angular.forEach($scope.query_p, function (q, index) {
                var m = q.metric_tags;
                if (!m) {
                    return;
                }
                if (!r.queries[index]) {
                    return;
                }
                angular.forEach(q.tags, function (key, tag) {
                    if (m[tag]) {
                        return;
                    }
                    delete r.queries[index].tags[tag];
                });
                if ($scope.filterSupport) {
                    _.each(r.queries[index].filters, function (f) {
                        if (m[f.tagk]) {
                            return;
                        }
                        delete r.queries[index].nGbFilters[f.tagk];
                        delete r.queries[index].gbFilters[f.tagk];
                        r.queries[index].filters = _.without(r.queries[index].filters, _.findWhere(r.queries[index].filters, { tagk: f.tagk }));
                    });
                }
            });
            r.prune();
            $location.search('b64', btoa(JSON.stringify(r)));
            $location.search('autods', $scope.autods ? undefined : 'false');
            $location.search('refresh', $scope.refresh ? 'true' : undefined);
            $location.search('normalize', $scope.normalize ? 'true' : undefined);
            var min = angular.isNumber($scope.min) ? $scope.min.toString() : null;
            var max = angular.isNumber($scope.max) ? $scope.max.toString() : null;
            $location.search('min', min);
            $location.search('max', max);
            $route.reload();
        };
        request = getRequest();
        if (!request.queries.length) {
            return;
        }
        var autods = $scope.autods ? '&autods=' + $('#chart').width() : '';
        function getMetricMeta(metric) {
            $http.get('/api/metadata/metrics?metric=' + encodeURIComponent(metric))
                .success(function (data) {
                $scope.meta[metric] = data;
            })
                .error(function (error) {
                console.log("Error getting metadata for metric " + metric);
            });
        }
        function get(noRunning) {
            $timeout.cancel(graphRefresh);
            if (!noRunning) {
                $scope.running = 'Running';
            }
            var autorate = '';
            $scope.meta = {};
            for (var i = 0; i < request.queries.length; i++) {
                if (request.queries[i].derivative == 'auto') {
                    autorate += '&autorate=' + i;
                }
                getMetricMeta(request.queries[i].metric);
            }
            _.each(request.queries, function (q, qIndex) {
                request.queries[qIndex].filters = _.map(q.filters, function (filter) {
                    var f = new Filter(filter);
                    if (f.filter && f.type) {
                        if (f.type == "auto") {
                            if (f.filter.indexOf("*") > -1) {
                                f.type = f.filter == "*" ? f.type = "wildcard" : "iwildcard";
                            }
                            else {
                                f.type = "literal_or";
                            }
                        }
                    }
                    return f;
                });
            });
            var min = angular.isNumber($scope.min) ? '&min=' + encodeURIComponent($scope.min.toString()) : '';
            var max = angular.isNumber($scope.max) ? '&max=' + encodeURIComponent($scope.max.toString()) : '';
            $scope.animate();
            $scope.queryTime = '';
            if (request.end && !isRel.exec(request.end)) {
                var t = moment.utc(request.end, moment.defaultFormat);
                $scope.queryTime = '&date=' + t.format('YYYY-MM-DD');
                $scope.queryTime += '&time=' + t.format('HH:mm');
            }
            $http.get('/api/graph?' + 'b64=' + encodeURIComponent(btoa(JSON.stringify(request))) + autods + autorate + min + max)
                .success(function (data) {
                $scope.result = data.Series;
                if ($scope.annotateEnabled) {
                    $scope.annotations = _.sortBy(data.Annotations, function (d) { return d.StartDate; });
                }
                if (!$scope.result) {
                    $scope.warning = 'No Results';
                }
                else {
                    $scope.warning = '';
                }
                $scope.queries = data.Queries;
                $scope.exprText = "";
                _.each($scope.queries, function (q, i) {
                    $scope.exprText += "$" + alphabet[i] + " = " + q + "\n";
                    if (i == $scope.queries.length - 1) {
                        $scope.exprText += "avg($" + alphabet[i] + ")";
                    }
                });
                $scope.running = '';
                $scope.error = '';
                var u = $location.absUrl();
                u = u.substr(0, u.indexOf('?')) + '?';
                u += 'b64=' + search.b64 + autods + autorate + min + max;
                $scope.url = u;
            })
                .error(function (error) {
                $scope.error = error;
                $scope.running = '';
            })
                .finally(function () {
                $scope.stop();
                if ($scope.refresh) {
                    graphRefresh = $timeout(function () { get(true); }, 5000);
                }
                ;
            });
        }
        ;
        get(false);
    }]);
bosunApp.directive('tsPopup', function () {
    return {
        restrict: 'E',
        scope: {
            url: '='
        },
        template: '<button class="btn btn-default" data-html="true" data-placement="bottom">embed</button>',
        link: function (scope, elem, attrs) {
            var button = $('button', elem);
            scope.$watch(attrs.url, function (url) {
                if (!url) {
                    return;
                }
                var text = '<input type="text" onClick="this.select();" readonly="readonly" value="&lt;a href=&quot;' + url + '&quot;&gt;&lt;img src=&quot;' + url + '&.png=png&quot;&gt;&lt;/a&gt;">';
                button.popover({
                    content: text
                });
            });
        }
    };
});
bosunApp.directive('tsAlertHistory', function () {
    return {
        templateUrl: '/partials/alerthistory.html'
    };
});
bosunControllers.controller('HistoryCtrl', ['$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        var search = $location.search();
        var keys = {};
        if (angular.isArray(search.key)) {
            angular.forEach(search.key, function (v) {
                keys[v] = true;
            });
        }
        else {
            keys[search.key] = true;
        }
        var params = Object.keys(keys).map(function (v) { return 'ak=' + encodeURIComponent(v); }).join('&');
        $http.get('/api/status?' + params + "&all=1")
            .success(function (data) {
            console.log(data);
            var selected_alerts = {};
            angular.forEach(data, function (v, ak) {
                if (!keys[ak]) {
                    return;
                }
                v.Events.map(function (h) { h.Time = moment.utc(h.Time); });
                angular.forEach(v.Events, function (h, i) {
                    if (i + 1 < v.Events.length) {
                        h.EndTime = v.Events[i + 1].Time;
                    }
                    else {
                        h.EndTime = moment.utc();
                    }
                });
                selected_alerts[ak] = {
                    History: v.Events.reverse()
                };
            });
            if (Object.keys(selected_alerts).length > 0) {
                $scope.alert_history = selected_alerts;
            }
            else {
                $scope.error = 'No Matching Alerts Found';
            }
        })
            .error(function (err) {
            $scope.error = err;
        });
    }]);
bosunControllers.controller('HostCtrl', ['$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        var search = $location.search();
        $scope.host = search.host;
        $scope.time = search.time;
        $scope.tab = search.tab || "stats";
        $scope.fsdata = [];
        $scope.metrics = [];
        var currentURL = $location.url();
        $scope.mlink = function (m) {
            var r = new Request();
            var q = new Query(false);
            q.metric = m;
            q.tags = { 'host': $scope.host };
            r.queries.push(q);
            return r;
        };
        $scope.setTab = function (t) {
            $location.search('tab', t);
            $scope.tab = t;
        };
        $http.get('/api/metric/host/' + $scope.host)
            .success(function (data) {
            $scope.metrics = data || [];
        });
        var start = moment().utc().subtract(parseDuration($scope.time));
        function parseDuration(v) {
            var pattern = /(\d+)(d|y|n|h|m|s)-ago/;
            var m = pattern.exec(v);
            return moment.duration(parseInt(m[1]), m[2].replace('n', 'M'));
        }
        $http.get('/api/metadata/get?tagk=host&tagv=' + encodeURIComponent($scope.host))
            .success(function (data) {
            $scope.metadata = _.filter(data, function (i) {
                return moment.utc(i.Time) > start;
            });
        });
        var autods = '&autods=100';
        var cpu_r = new Request();
        cpu_r.start = $scope.time;
        cpu_r.queries = [
            new Query(false, {
                metric: 'os.cpu',
                derivative: 'counter',
                tags: { host: $scope.host }
            })
        ];
        $http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(cpu_r)) + autods)
            .success(function (data) {
            if (!data.Series) {
                return;
            }
            data.Series[0].Name = 'Percent Used';
            $scope.cpu = data.Series;
        });
        var mem_r = new Request();
        mem_r.start = $scope.time;
        mem_r.queries.push(new Query(false, {
            metric: "os.mem.total",
            tags: { host: $scope.host }
        }));
        mem_r.queries.push(new Query(false, {
            metric: "os.mem.used",
            tags: { host: $scope.host }
        }));
        $http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(mem_r)) + autods)
            .success(function (data) {
            if (!data.Series) {
                return;
            }
            data.Series[1].Name = "Used";
            $scope.mem_total = Math.max.apply(null, data.Series[0].Data.map(function (d) { return d[1]; }));
            $scope.mem = [data.Series[1]];
        });
        var net_bytes_r = new Request();
        net_bytes_r.start = $scope.time;
        net_bytes_r.queries = [
            new Query(false, {
                metric: "os.net.bytes",
                rate: true,
                rateOptions: { counter: true, resetValue: 1 },
                tags: { host: $scope.host, iface: "*", direction: "*" }
            })
        ];
        $http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(net_bytes_r)) + autods)
            .success(function (data) {
            if (!data.Series) {
                return;
            }
            var tmp = [];
            var ifaceSeries = {};
            angular.forEach(data.Series, function (series, idx) {
                series.Data = series.Data.map(function (dp) { return [dp[0], dp[1] * 8]; });
                if (series.Tags.direction == "out") {
                    series.Data = series.Data.map(function (dp) { return [dp[0], dp[1] * -1]; });
                }
                if (!ifaceSeries.hasOwnProperty(series.Tags.iface)) {
                    ifaceSeries[series.Tags.iface] = [series];
                }
                else {
                    ifaceSeries[series.Tags.iface].push(series);
                    tmp.push(ifaceSeries[series.Tags.iface]);
                }
            });
            $scope.idata = tmp;
        });
        var fs_r = new Request();
        fs_r.start = $scope.time;
        fs_r.queries = [
            new Query(false, {
                metric: "os.disk.fs.space_total",
                tags: { host: $scope.host, disk: "*" }
            }),
            new Query(false, {
                metric: "os.disk.fs.space_used",
                tags: { host: $scope.host, disk: "*" }
            })
        ];
        $http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(fs_r)) + autods)
            .success(function (data) {
            if (!data.Series) {
                return;
            }
            var tmp = [];
            var fsSeries = {};
            angular.forEach(data.Series, function (series, idx) {
                var stat = series.Data[series.Data.length - 1][1];
                var prop = "";
                if (series.Metric == "os.disk.fs.space_total") {
                    prop = "total";
                }
                else {
                    prop = "used";
                }
                if (!fsSeries.hasOwnProperty(series.Tags.disk)) {
                    fsSeries[series.Tags.disk] = [series];
                    fsSeries[series.Tags.disk][prop] = stat;
                }
                else {
                    fsSeries[series.Tags.disk].push(series);
                    fsSeries[series.Tags.disk][prop] = stat;
                    tmp.push(fsSeries[series.Tags.disk]);
                }
            });
            $scope.fsdata = tmp;
        });
    }]);
bosunControllers.controller('IncidentCtrl', ['$scope', '$http', '$location', '$route', '$sce', function ($scope, $http, $location, $route, $sce) {
        var search = $location.search();
        var id = search.id;
        if (!id) {
            $scope.error = "must supply incident id as query parameter";
            return;
        }
        $http.get('/api/config')
            .success(function (data) {
            $scope.config_text = data;
        });
        $scope.action = function (type) {
            var key = encodeURIComponent($scope.state.AlertKey);
            return '/action?type=' + type + '&key=' + key;
        };
        $scope.loadTimelinePanel = function (v, i) {
            if (v.doneLoading && !v.error) {
                return;
            }
            v.error = null;
            v.doneLoading = false;
            if (i == $scope.lastNonUnknownAbnormalIdx) {
                v.subject = $scope.incident.Subject;
                v.body = $scope.body;
                v.doneLoading = true;
                return;
            }
            var ak = $scope.incident.AlertKey;
            var url = ruleUrl(ak, moment(v.Time));
            $http.post(url, $scope.config_text)
                .success(function (data) {
                v.subject = data.Subject;
                v.body = $sce.trustAsHtml(data.Body);
            })
                .error(function (error) {
                v.error = error;
            })
                .finally(function () {
                v.doneLoading = true;
            });
        };
        $scope.shown = {};
        $scope.collapse = function (i, v) {
            $scope.shown[i] = !$scope.shown[i];
            if ($scope.loadTimelinePanel && $scope.shown[i]) {
                $scope.loadTimelinePanel(v, i);
            }
        };
        $http.get('/api/incidents/events?id=' + id)
            .success(function (data) {
            $scope.incident = data;
            $scope.state = $scope.incident;
            $scope.actions = data.Actions;
            $scope.body = $sce.trustAsHtml(data.Body);
            $scope.events = data.Events.reverse();
            $scope.configLink = configUrl($scope.incident.AlertKey, moment.unix($scope.incident.LastAbnormalTime * 1000));
            for (var i = 0; i < $scope.events.length; i++) {
                var e = $scope.events[i];
                if (e.Status != 'normal' && e.Status != 'unknown') {
                    $scope.lastNonUnknownAbnormalIdx = i;
                    $scope.collapse(i, e); // Expand the panel of the current body
                    break;
                }
            }
            $scope.collapse;
        })
            .error(function (err) {
            $scope.error = err;
        });
    }]);
bosunControllers.controller('ItemsCtrl', ['$scope', '$http', function ($scope, $http) {
        $http.get('/api/metric')
            .success(function (data) {
            $scope.metrics = data;
        })
            .error(function (error) {
            $scope.status = 'Unable to fetch metrics: ' + error;
        });
        $http.get('/api/tagv/host?since=default')
            .success(function (data) {
            $scope.hosts = data;
        })
            .error(function (error) {
            $scope.status = 'Unable to fetch hosts: ' + error;
        });
    }]);
var Tag = (function () {
    function Tag() {
    }
    return Tag;
})();
var DP = (function () {
    function DP() {
    }
    return DP;
})();
bosunControllers.controller('PutCtrl', ['$scope', '$http', '$route', function ($scope, $http, $route) {
        $scope.tags = [new Tag];
        var dp = new DP;
        dp.k = moment().utc().format();
        $scope.dps = [dp];
        $http.get('/api/metric')
            .success(function (data) {
            $scope.metrics = data;
        })
            .error(function (error) {
            $scope.error = 'Unable to fetch metrics: ' + error;
        });
        $scope.Submit = function () {
            var data = [];
            var tags = {};
            angular.forEach($scope.tags, function (v, k) {
                if (v.k || v.v) {
                    tags[v.k] = v.v;
                }
            });
            angular.forEach($scope.dps, function (v, k) {
                if (v.k && v.v) {
                    var ts = parseInt(moment.utc(v.k, tsdbDateFormat).format('X'));
                    data.push({
                        metric: $scope.metric,
                        timestamp: ts,
                        value: parseFloat(v.v),
                        tags: tags
                    });
                }
            });
            $scope.running = 'submitting data...';
            $scope.success = '';
            $scope.error = '';
            $http.post('/api/put', data)
                .success(function () {
                $scope.running = '';
                $scope.success = 'Data Submitted';
            })
                .error(function (error) {
                $scope.running = '';
                $scope.error = error.error.message;
            });
        };
        $scope.AddTag = function () {
            var last = $scope.tags[$scope.tags.length - 1];
            if (last.k && last.v) {
                $scope.tags.push(new Tag);
            }
        };
        $scope.AddDP = function () {
            var last = $scope.dps[$scope.dps.length - 1];
            if (last.k && last.v) {
                var dp = new DP;
                dp.k = moment.utc(last.k, tsdbDateFormat).add(15, 'seconds').format();
                $scope.dps.push(dp);
            }
        };
        $scope.GetTagKByMetric = function () {
            $http.get('/api/tagk/' + $scope.metric)
                .success(function (data) {
                if (!angular.isArray(data)) {
                    return;
                }
                $scope.tags = [new Tag];
                for (var i = 0; i < data.length; i++) {
                    var t = new Tag;
                    t.k = data[i];
                    $scope.tags.push(t);
                }
            })
                .error(function (error) {
                $scope.error = 'Unable to fetch metrics: ' + error;
            });
        };
    }]);
bosunControllers.controller('SilenceCtrl', ['$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        var search = $location.search();
        $scope.start = search.start;
        $scope.end = search.end;
        $scope.duration = search.duration;
        $scope.alert = search.alert;
        $scope.hosts = search.hosts;
        $scope.tags = search.tags;
        $scope.edit = search.edit;
        $scope.forget = search.forget;
        $scope.user = getUser();
        $scope.message = search.message;
        if (!$scope.end && !$scope.duration) {
            $scope.duration = '1h';
        }
        function filter(data, startBefore, startAfter, endAfter, endBefore, limit) {
            var ret = {};
            var count = 0;
            _.each(data, function (v, name) {
                if (limit && count >= limit) {
                    return;
                }
                var s = moment(v.Start).utc();
                var e = moment(v.End).utc();
                if (startBefore && s > startBefore) {
                    return;
                }
                if (startAfter && s < startAfter) {
                    return;
                }
                if (endAfter && e < endAfter) {
                    return;
                }
                if (endBefore && e > endBefore) {
                    return;
                }
                ret[name] = v;
            });
            return ret;
        }
        function get() {
            $http.get('/api/silence/get')
                .success(function (data) {
                $scope.silences = [];
                var now = moment.utc();
                $scope.silences.push({
                    name: 'Active',
                    silences: filter(data, now, null, now, null, 0)
                });
                $scope.silences.push({
                    name: 'Upcoming',
                    silences: filter(data, null, now, null, null, 0)
                });
                $scope.silences.push({
                    name: 'Past',
                    silences: filter(data, null, null, null, now, 25)
                });
            })
                .error(function (error) {
                $scope.error = error;
            });
        }
        get();
        function getData() {
            var tags = ($scope.tags || '').split(',');
            if ($scope.hosts) {
                tags.push('host=' + $scope.hosts.split(/[ ,|]+/).join('|'));
            }
            tags = tags.filter(function (v) { return v != ""; });
            var data = {
                start: $scope.start,
                end: $scope.end,
                duration: $scope.duration,
                alert: $scope.alert,
                tags: tags.join(','),
                edit: $scope.edit,
                forget: $scope.forget ? 'true' : null,
                user: $scope.user,
                message: $scope.message
            };
            return data;
        }
        var any = search.start || search.end || search.duration || search.alert || search.hosts || search.tags || search.forget;
        var state = getData();
        $scope.change = function () {
            $scope.disableConfirm = true;
        };
        if (any) {
            $scope.error = null;
            $http.post('/api/silence/set', state)
                .success(function (data) {
                if (!data) {
                    data = { '(none)': false };
                }
                $scope.testSilences = data;
            })
                .error(function (error) {
                $scope.error = error;
            });
        }
        $scope.test = function () {
            setUser($scope.user);
            $location.search('start', $scope.start || null);
            $location.search('end', $scope.end || null);
            $location.search('duration', $scope.duration || null);
            $location.search('alert', $scope.alert || null);
            $location.search('hosts', $scope.hosts || null);
            $location.search('tags', $scope.tags || null);
            $location.search('forget', $scope.forget || null);
            $location.search('message', $scope.message || null);
            $route.reload();
        };
        $scope.confirm = function () {
            $scope.error = null;
            $scope.testSilences = null;
            $scope.edit = null;
            $location.search('edit', null);
            state.confirm = 'true';
            $http.post('/api/silence/set', state)
                .error(function (error) {
                $scope.error = error;
            })
                .finally(get);
        };
        $scope.clear = function (id) {
            if (!window.confirm('Clear this silence?')) {
                return;
            }
            $scope.error = null;
            $http.post('/api/silence/clear?id=' + id, {})
                .error(function (error) {
                $scope.error = error;
            })
                .finally(get);
        };
        $scope.time = function (v) {
            var m = moment(v).utc();
            return m.format();
        };
    }]);
bosunApp.directive('tsAckGroup', ['$location', '$timeout', function ($location, $timeout) {
        return {
            scope: {
                ack: '=',
                groups: '=tsAckGroup',
                schedule: '=',
                timeanddate: '='
            },
            templateUrl: '/partials/ackgroup.html',
            link: function (scope, elem, attrs) {
                scope.canAckSelected = scope.ack == 'Needs Acknowledgement';
                scope.panelClass = scope.$parent.panelClass;
                scope.btoa = scope.$parent.btoa;
                scope.encode = scope.$parent.encode;
                scope.shown = {};
                scope.collapse = function (i) {
                    scope.shown[i] = !scope.shown[i];
                    if (scope.shown[i] && scope.groups[i].Children.length == 1) {
                        $timeout(function () {
                            scope.$broadcast("onOpen", i);
                        }, 0);
                    }
                };
                scope.click = function ($event, idx) {
                    scope.collapse(idx);
                    if ($event.shiftKey && scope.schedule.checkIdx != undefined) {
                        var checked = scope.groups[scope.schedule.checkIdx].checked;
                        var start = Math.min(idx, scope.schedule.checkIdx);
                        var end = Math.max(idx, scope.schedule.checkIdx);
                        for (var i = start; i <= end; i++) {
                            if (i == idx) {
                                continue;
                            }
                            scope.groups[i].checked = checked;
                        }
                    }
                    scope.schedule.checkIdx = idx;
                    scope.update();
                };
                scope.select = function (checked) {
                    for (var i = 0; i < scope.groups.length; i++) {
                        scope.groups[i].checked = checked;
                    }
                    scope.update();
                };
                scope.update = function () {
                    scope.canCloseSelected = true;
                    scope.canForgetSelected = true;
                    scope.anySelected = false;
                    for (var i = 0; i < scope.groups.length; i++) {
                        var g = scope.groups[i];
                        if (!g.checked) {
                            continue;
                        }
                        scope.anySelected = true;
                        if (g.Active && g.Status != 'unknown' && g.Status != 'error') {
                            scope.canCloseSelected = false;
                        }
                        if (g.Status != 'unknown') {
                            scope.canForgetSelected = false;
                        }
                    }
                };
                scope.multiaction = function (type) {
                    var keys = [];
                    angular.forEach(scope.groups, function (group) {
                        if (!group.checked) {
                            return;
                        }
                        if (group.AlertKey) {
                            keys.push(group.AlertKey);
                        }
                        angular.forEach(group.Children, function (child) {
                            keys.push(child.AlertKey);
                        });
                    });
                    scope.$parent.setKey("action-keys", keys);
                    $location.path("action");
                    $location.search("type", type);
                };
                scope.history = function () {
                    var url = '/history?';
                    angular.forEach(scope.groups, function (group) {
                        if (!group.checked) {
                            return;
                        }
                        if (group.AlertKey) {
                            url += '&key=' + encodeURIComponent(group.AlertKey);
                        }
                        angular.forEach(group.Children, function (child) {
                            url += '&key=' + encodeURIComponent(child.AlertKey);
                        });
                    });
                    return url;
                };
            }
        };
    }]);
bosunApp.directive('tsState', ['$sce', '$http', function ($sce, $http) {
        return {
            templateUrl: '/partials/alertstate.html',
            link: function (scope, elem, attrs) {
                var myIdx = attrs["tsGrp"];
                scope.currentStatus = attrs["tsGrpstatus"];
                scope.name = scope.child.AlertKey;
                scope.state = scope.child.State;
                scope.action = function (type) {
                    var key = encodeURIComponent(scope.name);
                    return '/action?type=' + type + '&key=' + key;
                };
                var loadedBody = false;
                scope.toggle = function () {
                    scope.show = !scope.show;
                    if (scope.show && !loadedBody) {
                        scope.state.Body = "loading...";
                        loadedBody = true;
                        $http.get('/api/status?ak=' + scope.child.AlertKey)
                            .success(function (data) {
                            var body = data[scope.child.AlertKey].Body;
                            scope.state.Body = $sce.trustAsHtml(body);
                        })
                            .error(function (err) {
                            scope.state.Body = "Error loading template body: " + err;
                        });
                    }
                };
                scope.$on('onOpen', function (e, i) {
                    if (i == myIdx) {
                        scope.toggle();
                    }
                });
                scope.zws = function (v) {
                    if (!v) {
                        return '';
                    }
                    return v.replace(/([,{}()])/g, '$1\u200b');
                };
                scope.state.Touched = moment(scope.state.Touched).utc();
                angular.forEach(scope.state.Events, function (v, k) {
                    v.Time = moment(v.Time).utc();
                });
                scope.state.last = scope.state.Events[scope.state.Events.length - 1];
                if (scope.state.Actions && scope.state.Actions.length > 0) {
                    scope.state.LastAction = scope.state.Actions[scope.state.Actions.length - 1];
                }
                scope.state.RuleUrl = '/config?' +
                    'alert=' + encodeURIComponent(scope.state.Alert) +
                    '&fromDate=' + encodeURIComponent(scope.state.last.Time.format("YYYY-MM-DD")) +
                    '&fromTime=' + encodeURIComponent(scope.state.last.Time.format("HH:mm"));
                var groups = [];
                angular.forEach(scope.state.Group, function (v, k) {
                    groups.push(k + "=" + v);
                });
                if (groups.length > 0) {
                    scope.state.RuleUrl += '&template_group=' + encodeURIComponent(groups.join(','));
                }
                scope.state.Body = $sce.trustAsHtml(scope.state.Body);
            }
        };
    }]);
bosunApp.directive('tsNote', function () {
    return {
        restrict: 'E',
        templateUrl: '/partials/note.html'
    };
});
bosunApp.directive('tsAck', function () {
    return {
        restrict: 'E',
        templateUrl: '/partials/ack.html'
    };
});
bosunApp.directive('tsClose', function () {
    return {
        restrict: 'E',
        templateUrl: '/partials/close.html'
    };
});
bosunApp.directive('tsForget', function () {
    return {
        restrict: 'E',
        templateUrl: '/partials/forget.html'
    };
});
bosunApp.directive('tsPurge', function () {
    return {
        restrict: 'E',
        templateUrl: '/partials/purge.html'
    };
});
bosunApp.directive('tsForceClose', function () {
    return {
        restrict: 'E',
        templateUrl: '/partials/forceClose.html'
    };
});
