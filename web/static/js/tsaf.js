/// <reference path="angular.d.ts" />
/// <reference path="angular-route.d.ts" />
/// <reference path="angular-sanitize.d.ts" />
/// <reference path="bootstrap.d.ts" />
/// <reference path="moment.d.ts" />
/// <reference path="rickshaw.d.ts" />
/// <reference path="d3.d.ts" />
var tsafApp = angular.module('tsafApp', [
    'ngRoute',
    'tsafControllers',
    'mgcrea.ngStrap',
    'ngSanitize'
]);

tsafApp.config([
    '$routeProvider', '$locationProvider', function ($routeProvider, $locationProvider) {
        $locationProvider.html5Mode(true);
        $routeProvider.when('/', {
            title: 'Dashboard',
            templateUrl: 'partials/dashboard.html',
            controller: 'DashboardCtrl'
        }).when('/items', {
            title: 'Items',
            templateUrl: 'partials/items.html',
            controller: 'ItemsCtrl'
        }).when('/expr', {
            title: 'Expression',
            templateUrl: 'partials/expr.html',
            controller: 'ExprCtrl'
        }).when('/egraph', {
            title: 'Expression Graph',
            templateUrl: 'partials/egraph.html',
            controller: 'EGraphCtrl'
        }).when('/graph', {
            title: 'Graph',
            templateUrl: 'partials/graph.html',
            controller: 'GraphCtrl'
        }).when('/host', {
            title: 'Host View',
            templateUrl: 'partials/host.html',
            controller: 'HostCtrl',
            reloadOnSearch: false
        }).when('/rule', {
            title: 'Rule',
            templateUrl: 'partials/rule.html',
            controller: 'RuleCtrl'
        }).when('/silence', {
            title: 'Silence',
            templateUrl: 'partials/silence.html',
            controller: 'SilenceCtrl'
        }).when('/test_template', {
            title: 'Test Template',
            templateUrl: 'partials/test_template.html',
            controller: 'TestTemplateCtrl'
        }).when('/config', {
            title: 'Configuration',
            templateUrl: 'partials/config.html',
            controller: 'ConfigCtrl'
        }).when('/action', {
            title: 'Action',
            templateUrl: 'partials/action.html',
            controller: 'ActionCtrl'
        }).otherwise({
            redirectTo: '/'
        });
    }]);

tsafApp.run([
    '$location', '$rootScope', function ($location, $rootScope) {
        $rootScope.$on('$routeChangeSuccess', function (event, current, previous) {
            $rootScope.title = current.$$route.title;
        });
    }]);

var tsafControllers = angular.module('tsafControllers', []);

tsafControllers.controller('TsafCtrl', [
    '$scope', '$route', '$http', function ($scope, $route, $http) {
        $scope.active = function (v) {
            if (!$route.current) {
                return null;
            }
            if ($route.current.loadedTemplateUrl == 'partials/' + v + '.html') {
                return { active: true };
            }
            return null;
        };
        $scope.json = function (v) {
            return JSON.stringify(v, null, '  ');
        };
        $scope.btoa = function (v) {
            return btoa(v);
        };
        $scope.zws = function (v) {
            return v.replace(/([,{}()])/g, '$1\u200b');
        };
        $scope.encode = function (v) {
            return encodeURIComponent(v);
        };
        $scope.req_from_m = function (m) {
            var r = new Request();
            var q = new Query();
            q.metric = m;
            r.queries.push(q);
            return r;
        };
        $scope.refresh = function () {
            $http.get('/api/alerts').success(function (data) {
                angular.forEach(data.Status, function (v, k) {
                    v.Touched = moment(v.Touched).utc();
                    angular.forEach(v.History, function (v, k) {
                        v.Time = moment(v.Time).utc();
                    });
                    v.last = v.History[v.History.length - 1];
                });
                $scope.schedule = data;
                $scope.timeanddate = data.TimeAndDate;
            });
        };
    }]);

tsafControllers.controller('DashboardCtrl', [
    '$scope', function ($scope) {
        $scope.refresh();
    }]);

tsafControllers.controller('ItemsCtrl', [
    '$scope', '$http', function ($scope, $http) {
        $http.get('/api/metric').success(function (data) {
            $scope.metrics = data;
        }).error(function (error) {
            $scope.status = 'Unable to fetch metrics: ' + error;
        });
        $http.get('/api/tagv/host').success(function (data) {
            $scope.hosts = data;
        }).error(function (error) {
            $scope.status = 'Unable to fetch hosts: ' + error;
        });
    }]);

tsafControllers.controller('ExprCtrl', [
    '$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        var current = $location.hash();
        try  {
            current = atob(current);
        } catch (e) {
            current = '';
        }
        if (!current) {
            $location.hash(btoa('avg(q("avg:rate:os.cpu{host=ny-devtsaf01}", "5m", "")) > 80'));
            return;
        }
        $scope.expr = current;
        $scope.running = current;
        $http.get('/api/expr?q=' + encodeURIComponent(current)).success(function (data) {
            $scope.result = data.Results;
            $scope.queries = data.Queries;
            $scope.result_type = data.Type;
            $scope.running = '';
        }).error(function (error) {
            $scope.error = error;
            $scope.running = '';
        });
        $scope.set = function () {
            $location.hash(btoa($scope.expr));
            $route.reload();
        };
    }]);

tsafControllers.controller('EGraphCtrl', [
    '$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        var search = $location.search();
        var current = search.q;
        try  {
            current = atob(current);
        } catch (e) {
        }
        $scope.bytes = search.bytes == 'true';
        $scope.renderers = ['area', 'bar', 'line', 'scatterplot'];
        $scope.render = search.render || 'line';
        if (!current) {
            $location.search('q', btoa('q("avg:rate:os.cpu{host=ny-devtsaf01}", "5m", "")'));
            return;
        }
        $scope.expr = current;
        $scope.running = current;
        var width = $('.chart').width();
        $http.get('/api/egraph?q=' + encodeURIComponent(current) + '&autods=' + width).success(function (data) {
            $scope.result = data;
            if ($scope.result.length == 0) {
                $scope.warning = 'No Results';
            } else {
                $scope.warning = '';
            }
            $scope.running = '';
        }).error(function (error) {
            $scope.error = error;
            $scope.running = '';
        });
        $scope.set = function () {
            $location.search('q', btoa($scope.expr));
            $location.search('render', $scope.render);
            $location.search('bytes', $scope.bytes ? 'true' : undefined);
            $route.reload();
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

var Query = (function () {
    function Query(q) {
        this.aggregator = q && q.aggregator || 'sum';
        this.metric = q && q.metric || '';
        this.rate = q && q.rate || false;
        this.derivative = q && q.derivative || 'counter';
        this.rateOptions = q && q.rateOptions || new RateOptions;
        this.ds = q && q.ds || '';
        this.dstime = q && q.dstime || '';
        this.tags = q && q.tags || new TagSet;
        this.setDs();
        this.setDerivative();
    }
    Query.prototype.setDs = function () {
        if (this.dstime && this.ds) {
            this.downsample = this.dstime + '-' + this.ds;
        } else {
            this.downsample = '';
        }
    };
    Query.prototype.setDerivative = function () {
        var max = this.rateOptions.counterMax;
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

tsafControllers.controller('GraphCtrl', [
    '$scope', '$http', '$location', '$route', '$timeout', function ($scope, $http, $location, $route, $timeout) {
        $scope.aggregators = ["sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
        $scope.dsaggregators = ["", "sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
        $scope.rate_options = ["gauge", "counter", "rate"];
        var search = $location.search();
        var j = search.json;
        if (search.b64) {
            j = atob(search.b64);
        }
        var request = j ? JSON.parse(j) : new Request;
        $scope.index = parseInt($location.hash()) || 0;
        $scope.tagvs = [];
        $scope.sorted_tagks = [];
        $scope.query_p = [];
        angular.forEach(request.queries, function (q, i) {
            $scope.query_p[i] = new Query(q);
        });
        $scope.start = request.start;
        $scope.end = request.end;
        $scope.autods = search.autods != 'false';
        $scope.refresh = search.refresh == 'true';
        $scope.AddTab = function () {
            $scope.index = $scope.query_p.length;
            $scope.query_p.push(new Query);
        };
        $scope.setIndex = function (i) {
            $scope.index = i;
        };
        $scope.GetTagKByMetric = function (index) {
            $scope.tagvs[index] = new TagV;
            if ($scope.query_p[index].metric) {
                $http.get('/api/tagk/' + $scope.query_p[index].metric).success(function (data) {
                    if (!angular.isArray(data)) {
                        return;
                    }
                    var tags = {};
                    for (var i = 0; i < data.length; i++) {
                        if ($scope.query_p[index].tags) {
                            tags[data[i]] = $scope.query_p[index].tags[data[i]] || '';
                        } else {
                            tags[data[i]] = '';
                        }
                        GetTagVs(data[i], index);
                    }
                    $scope.query_p[index].tags = tags;

                    // Make sure host is always the first tag.
                    $scope.sorted_tagks[index] = Object.keys(tags);
                    $scope.sorted_tagks[index].sort(function (a, b) {
                        if (a == 'host') {
                            return 1;
                        } else if (b == 'host') {
                            return -1;
                        }
                        return a.localeCompare(b);
                    }).reverse();
                }).error(function (error) {
                    $scope.error = 'Unable to fetch metrics: ' + error;
                });
            }
        };
        if ($scope.query_p.length == 0) {
            $scope.AddTab();
        }
        $http.get('/api/metric').success(function (data) {
            $scope.metrics = data;
        }).error(function (error) {
            $scope.error = 'Unable to fetch metrics: ' + error;
        });

        function GetTagVs(k, index) {
            $http.get('/api/tagv/' + k + '/' + $scope.query_p[index].metric).success(function (data) {
                data.sort();
                $scope.tagvs[index][k] = data;
            }).error(function (error) {
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
                var q = new Query(p);
                var tags = q.tags;
                q.tags = new TagSet;
                angular.forEach(tags, function (v, k) {
                    if (v && k) {
                        q.tags[k] = v;
                    }
                });
                request.queries.push(q);
            });
            return request;
        }
        $scope.Query = function () {
            var r = getRequest();
            r.prune();
            $location.search('b64', btoa(JSON.stringify(r)));
            $location.search('autods', $scope.autods ? undefined : 'false');
            $location.search('refresh', $scope.refresh ? 'true' : undefined);
            $route.reload();
        };
        request = getRequest();
        if (!request.queries.length) {
            return;
        }
        var autods = $scope.autods ? autods = '&autods=' + $('.chart').width() : '';
        function get() {
            $timeout.cancel(graphRefresh);
            $scope.running = 'Running';
            $http.get('/api/graph?' + 'b64=' + btoa(JSON.stringify(request)) + autods).success(function (data) {
                $scope.result = data.Series;
                if (!$scope.result) {
                    $scope.warning = 'No Results';
                } else {
                    $scope.warning = '';
                }
                $scope.queries = data.Queries;
                $scope.running = '';
                $scope.error = '';
                var u = $location.absUrl();
                u = u.substr(0, u.indexOf('?')) + '?';
                u += 'b64=' + search.b64 + autods;
                $scope.url = u;
            }).error(function (error) {
                $scope.error = error;
                $scope.running = '';
            }).finally(function () {
                if ($scope.refresh) {
                    graphRefresh = $timeout(get, 5000);
                }
                ;
            });
        }
        ;
        get();
    }]);

tsafControllers.controller('HostCtrl', [
    '$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        var search = $location.search();
        $scope.host = search.host;
        $scope.time = search.time;
        $scope.tab = search.tab || "stats";
        $scope.idata = [];
        $scope.fsdata = [];
        $scope.fs_current = [];
        $scope.metrics = [];
        $scope.mlink = function (m) {
            var r = new Request();
            var q = new Query();
            q.metric = m;
            q.tags = { 'host': $scope.host };
            r.queries.push(q);
            return r;
        };
        $scope.setTab = function (t) {
            $location.search('tab', t);
            $scope.tab = t;
        };
        $http.get('/api/metric/host/' + $scope.host).success(function (data) {
            $scope.metrics = data || [];
        });
        var cpu_r = new Request();
        cpu_r.start = $scope.time;
        cpu_r.queries = [
            new Query({
                metric: 'os.cpu',
                derivative: 'counter',
                tags: { host: $scope.host }
            })
        ];
        var width = 500;
        $http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(cpu_r)) + '&autods=' + width).success(function (data) {
            data.Series[0].name = 'Percent Used';
            $scope.cpu = data.Series;
        });
        $http.get('/api/tagv/iface/os.net.bytes?host=' + $scope.host).success(function (data) {
            $scope.interfaces = data;
            angular.forEach($scope.interfaces, function (i) {
                var net_bytes_r = new Request();
                net_bytes_r.start = $scope.time;
                net_bytes_r.queries = [
                    new Query({
                        metric: "os.net.bytes",
                        rate: true,
                        tags: { host: $scope.host, iface: i, direction: "*" }
                    })
                ];
                $http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(net_bytes_r)) + '&autods=' + width).success(function (data) {
                    angular.forEach(data.Series, function (d) {
                        d.data = d.data.map(function (dp) {
                            return { x: dp.x, y: dp.y * 8 };
                        });
                        if (d.name.indexOf("direction=out") != -1) {
                            d.data = d.data.map(function (dp) {
                                return { x: dp.x, y: dp.y * -1 };
                            });
                            d.name = "out";
                        } else {
                            d.name = "in";
                        }
                    });
                    $scope.idata[$scope.interfaces.indexOf(i)] = { name: i, data: data.Series };
                });
            });
        });
        $http.get('/api/tagv/disk/os.disk.fs.space_total?host=' + $scope.host).success(function (data) {
            $scope.fs = data;
            angular.forEach($scope.fs, function (i) {
                if (i == '/dev/shm') {
                    return;
                }
                var fs_r = new Request();
                fs_r.start = $scope.time;
                fs_r.queries.push(new Query({
                    metric: "os.disk.fs.space_total",
                    tags: { host: $scope.host, disk: i }
                }));
                fs_r.queries.push(new Query({
                    metric: "os.disk.fs.space_used",
                    tags: { host: $scope.host, disk: i }
                }));
                $http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(fs_r)) + '&autods=' + width).success(function (data) {
                    data.Series[1].name = "Used";
                    $scope.fsdata[$scope.fs.indexOf(i)] = { name: i, data: [data.Series[1]] };
                    var total = Math.max.apply(null, data.Series[0].data.map(function (d) {
                        return d.y;
                    }));
                    var c_val = data.Series[1].data.slice(-1)[0].y;
                    var percent_used = c_val / total * 100;
                    $scope.fs_current[$scope.fs.indexOf(i)] = {
                        total: total,
                        c_val: c_val,
                        percent_used: percent_used
                    };
                });
            });
        });
        var mem_r = new Request();
        mem_r.start = $scope.time;
        mem_r.queries.push(new Query({
            metric: "os.mem.total",
            tags: { host: $scope.host }
        }));
        mem_r.queries.push(new Query({
            metric: "os.mem.used",
            tags: { host: $scope.host }
        }));
        $http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(mem_r)) + '&autods=' + width).success(function (data) {
            data.Series[1].name = "Used";
            $scope.mem_total = Math.max.apply(null, data.Series[0].data.map(function (d) {
                return d.y;
            }));
            $scope.mem = [data.Series[1]];
        });
    }]);

tsafControllers.controller('RuleCtrl', [
    '$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        var search = $location.search();
        var current = search.rule;
        $scope.date = search.date || '';
        $scope.time = search.time || '';
        try  {
            current = atob(current);
        } catch (e) {
            current = '';
        }
        if (!current) {
            var def = '$t = "5m"\n' + 'crit = avg(q("avg:os.cpu", $t, "")) > 10';
            $location.search('rule', btoa(def));
            return;
        }
        $scope.expr = current;
        $scope.running = current;
        $http.get('/api/rule?' + 'rule=' + encodeURIComponent(current) + '&date=' + encodeURIComponent($scope.date) + '&time=' + encodeURIComponent($scope.time)).success(function (data) {
            $scope.result = data.Results;
            $scope.queries = data.Queries;
            $scope.running = '';
        }).error(function (error) {
            $scope.error = error;
            $scope.running = '';
        });
        $scope.shiftEnter = function ($event) {
            if ($event.keyCode == 13 && $event.shiftKey) {
                $scope.set();
            }
        };
        $scope.set = function () {
            $location.search('rule', btoa($scope.expr));
            $location.search('date', $scope.date || null);
            $location.search('time', $scope.time || null);
            $route.reload();
        };
    }]);

tsafControllers.controller('TestTemplateCtrl', [
    '$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        var search = $location.search();
        var current = search.config;
        try  {
            current = atob(current);
        } catch (e) {
            current = '';
        }
        if (!current) {
            var def = 'template test {\n' + '    body = `<h1>Name: {{.Alert.Name}}</h1>`\n' + '    subject = `{{.Last.Status}}: {{.Alert.Name}}: {{.E .Alert.Vars.q}} on {{.Group.host}}`\n' + '}\n\n' + 'alert test {\n' + '    template = test\n' + '    $t = "5m"\n' + '    $q = avg(q("avg:rate{counter,,1}:os.cpu{host=*}", $t, ""))\n' + '    crit = $q > 10\n' + '}';
            $location.search('config', btoa(def));
            return;
        }
        $scope.config = current;
        $scope.running = "Running";
        $http.get('/api/template?' + 'config=' + encodeURIComponent($scope.config)).success(function (data) {
            $scope.subject = data.Subject;
            $scope.body = data.Body;
            $scope.running = '';
        }).error(function (error) {
            $scope.error = error;
            $scope.running = '';
        });
        $scope.shiftEnter = function ($event) {
            if ($event.keyCode == 13 && $event.shiftKey) {
                $scope.set();
            }
        };
        $scope.set = function () {
            $location.search('config', btoa($scope.config));
            $route.reload();
        };
    }]);

tsafControllers.controller('ConfigCtrl', [
    '$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        var search = $location.search();
        var current = search.config_text;
        var line_re = /test:(\d+)/;
        try  {
            current = atob(current);
        } catch (e) {
            current = '';
        }
        if (!current) {
            var def = '';
            $http.get('/api/config').success(function (data) {
                def = data;
            }).finally(function () {
                $location.search('config_text', btoa(def));
            });
            return;
        }
        $scope.config_text = current;
        $scope.set = function () {
            $scope.result = null;
            $scope.line = null;
            $http.get('/api/config_test?config_text=' + encodeURIComponent($scope.config_text)).success(function (data) {
                if (data == "") {
                    $scope.result = "Valid";
                } else {
                    $scope.result = data;
                    var m = data.match(line_re);
                    if (angular.isArray(m) && (m.length > 1)) {
                        $scope.line = m[1];
                    }
                }
            }).error(function (error) {
                $scope.error = error || 'Error';
            });
        };
        $scope.set();
    }]);

tsafControllers.controller('SilenceCtrl', [
    '$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        var search = $location.search();
        $scope.start = search.start;
        $scope.end = search.end;
        $scope.duration = search.duration;
        $scope.alert = search.alert;
        $scope.hosts = search.hosts;
        $scope.tags = search.tags;
        $scope.edit = search.edit;
        function get() {
            $http.get('/api/silence/get').success(function (data) {
                $scope.silences = data;
            }).error(function (error) {
                $scope.error = error;
            });
        }
        get();
        function getData() {
            var tags = ($scope.tags || '').split(',');
            if ($scope.hosts) {
                tags.push('host=' + $scope.hosts.split(/[ ,|]+/).join('|'));
            }
            tags = tags.filter(function (v) {
                return v != "";
            });
            var data = {
                start: $scope.start,
                end: $scope.end,
                duration: $scope.duration,
                alert: $scope.alert,
                tags: tags.join(','),
                edit: $scope.edit
            };
            return data;
        }
        var any = search.start || search.end || search.duration || search.alert || search.hosts || search.tags;
        var state = getData();
        $scope.change = function () {
            $scope.disableConfirm = true;
        };
        if (any) {
            $scope.error = null;
            $http.post('/api/silence/set', state).success(function (data) {
                if (data.length == 0) {
                    data = [{ Name: '(none)' }];
                }
                $scope.testSilences = data;
            }).error(function (error) {
                $scope.error = error;
            });
        }
        $scope.test = function () {
            $location.search('start', $scope.start || null);
            $location.search('end', $scope.end || null);
            $location.search('duration', $scope.duration || null);
            $location.search('alert', $scope.alert || null);
            $location.search('hosts', $scope.hosts || null);
            $location.search('tags', $scope.tags || null);
            $route.reload();
        };
        $scope.confirm = function () {
            $scope.error = null;
            $scope.testSilences = null;
            state.confirm = "true";
            $http.post('/api/silence/set', state).error(function (error) {
                $scope.error = error;
            }).finally(function () {
                $scope.testSilences = null;
                get();
            });
        };
        $scope.clear = function (id) {
            if (!window.confirm('Clear this silence?')) {
                return;
            }
            $scope.error = null;
            $http.post('/api/silence/clear', { id: id }).error(function (error) {
                $scope.error = error;
            }).finally(function () {
                get();
            });
        };
        $scope.time = function (v) {
            var m = moment(v).utc();
            return m.format(timeFormat);
        };
    }]);

tsafControllers.controller('ActionCtrl', [
    '$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        var search = $location.search();
        $scope.type = search.type;
        if (!angular.isArray(search.key)) {
            $scope.keys = [search.key];
        } else {
            $scope.keys = search.key;
        }
        $scope.submit = function () {
            var data = {
                type: $scope.type,
                user: $scope.user,
                message: $scope.message,
                key: $scope.keys[0]
            };
            $http.post('/api/action', data).success(function (data) {
                $location.url('/');
            }).error(function (error) {
                alert(error);
            });
        };
    }]);

tsafApp.directive('tsResults', function () {
    return {
        templateUrl: '/partials/results.html'
    };
});

tsafApp.directive('tsAckGroup', function () {
    return {
        scope: {
            ack: '=',
            groups: '=tsAckGroup',
            schedule: '=schedule'
        },
        templateUrl: '/partials/ackgroup.html',
        link: function (scope, elem, attrs) {
            scope.panelClass = function (status) {
                switch (status) {
                    case "critical":
                        return "panel-danger";
                    case "unknown":
                        return "panel-info";
                    case "warning":
                        return "panel-warning";
                    default:
                        return "panel-default";
                }
            };
            scope.shown = {};
            scope.collapse = function (i) {
                scope.shown[i] = !scope.shown[i];
            };
        }
    };
});

tsafApp.directive('tsState', function () {
    return {
        templateUrl: '/partials/alertstate.html',
        link: function (scope, elem, attrs) {
            scope.action = function (type) {
                var key = encodeURIComponent(scope.name);
                return '/action?type=' + type + '&key=' + key;
            };
        }
    };
});

tsafApp.directive('tsAck', function () {
    return {
        restrict: 'E',
        templateUrl: '/partials/ack.html'
    };
});

tsafApp.directive('tsClose', function () {
    return {
        restrict: 'E',
        templateUrl: '/partials/close.html'
    };
});

tsafApp.directive('tsForget', function () {
    return {
        restrict: 'E',
        templateUrl: '/partials/forget.html'
    };
});

var timeFormat = 'YYYY-MM-DD HH:mm:ss ZZ';

tsafApp.directive("tsTime", function () {
    return {
        link: function (scope, elem, attrs) {
            scope.$watch(attrs.tsTime, function (v) {
                var m = moment(v).utc();
                var el = document.createElement('a');
                el.innerText = m.format(timeFormat) + ' (' + m.fromNow() + ')';
                el.href = 'http://www.timeanddate.com/worldclock/converted.html?iso=';
                el.href += m.format('YYYYMMDDTHHmm');
                el.href += '&p1=0';
                angular.forEach(scope.timeanddate, function (v, k) {
                    el.href += '&p' + (k + 2) + '=' + v;
                });
                elem.html(el);
            });
        }
    };
});

tsafApp.directive("tsSince", function () {
    return {
        link: function (scope, elem, attrs) {
            scope.$watch(attrs.tsSince, function (v) {
                var m = moment(v).utc();
                elem.text(m.fromNow());
            });
        }
    };
});

tsafApp.directive("tsRickshaw", [
    '$filter', function ($filter) {
        return {
            //templateUrl: '/partials/rickshaw.html',
            link: function (scope, elem, attrs) {
                scope.$watch(attrs.tsRickshaw, function (v) {
                    if (!angular.isArray(v) || v.length == 0) {
                        return;
                    }
                    elem[0].innerHTML = '<div class="row"><div class="col-lg-12"><div class="y_axis"></div><div class="rgraph"></div></div></div><div class="row"><div class="col-lg-12"><div class="rlegend"></div></div></div>';
                    var palette = new Rickshaw.Color.Palette();
                    angular.forEach(v, function (i) {
                        if (!i.color) {
                            i.color = palette.color();
                        }
                    });
                    var rgraph = angular.element('.rgraph', elem);
                    var graph_options = {
                        element: rgraph[0],
                        height: rgraph.height(),
                        min: 'auto',
                        series: v,
                        renderer: 'line',
                        interpolation: 'linear'
                    };
                    if (attrs.max) {
                        graph_options.max = attrs.max;
                    }
                    if (attrs.renderer) {
                        graph_options.renderer = attrs.renderer;
                    }
                    var graph = new Rickshaw.Graph(graph_options);
                    var x_axis = new Rickshaw.Graph.Axis.Time({
                        graph: graph,
                        timeFixture: new Rickshaw.Fixtures.Time()
                    });
                    var y_axis = new Rickshaw.Graph.Axis.Y({
                        graph: graph,
                        orientation: 'left',
                        tickFormat: function (y) {
                            var o = d3.formatPrefix(y);

                            // The precision arg to d3.formatPrefix seems broken, so using round
                            // http://stackoverflow.com/questions/10310613/variable-precision-in-d3-format
                            return d3.round(o.scale(y), 2) + o.symbol;
                        },
                        element: angular.element('.y_axis', elem)[0]
                    });
                    if (attrs.bytes == "true") {
                        y_axis.tickFormat = function (y) {
                            return $filter('bytes')(y);
                        };
                    }
                    graph.render();
                    var fmter = 'nfmt';
                    if (attrs.bytes == 'true') {
                        fmter = 'bytes';
                    } else if (attrs.bits == 'true') {
                        fmter = 'bits';
                    }
                    var fmt = $filter(fmter);
                    var legend = angular.element('.rlegend', elem)[0];
                    var Hover = Rickshaw.Class.create(Rickshaw.Graph.HoverDetail, {
                        render: function (args) {
                            legend.innerHTML = args.formattedXValue;
                            args.detail.sort(function (a, b) {
                                return a.order - b.order;
                            }).forEach(function (d) {
                                var line = document.createElement('div');
                                line.className = 'rline';
                                var swatch = document.createElement('div');
                                swatch.className = 'rswatch';
                                swatch.style.backgroundColor = d.series.color;
                                var label = document.createElement('div');
                                label.className = 'rlabel';
                                label.innerHTML = d.name + ": " + fmt(d.formattedYValue);
                                line.appendChild(swatch);
                                line.appendChild(label);
                                legend.appendChild(line);
                                var dot = document.createElement('div');
                                dot.className = 'dot';
                                dot.style.top = graph.y(d.value.y0 + d.value.y) + 'px';
                                dot.style.borderColor = d.series.color;
                                this.element.appendChild(dot);
                                dot.className = 'dot active';
                                this.show();
                            }, this);
                        }
                    });
                    var hover = new Hover({ graph: graph });

                    //Simulate a movemove so the hover appears on load
                    var e = document.createEvent('MouseEvents');
                    e.initEvent('mousemove', true, false);
                    rgraph[0].children[0].dispatchEvent(e);
                });
            }
        };
    }]);

tsafApp.directive("tooltip", function () {
    return {
        link: function (scope, elem, attrs) {
            angular.element(elem[0]).tooltip({ placement: "bottom" });
        }
    };
});

tsafApp.directive('tsLine', function () {
    return {
        link: function (scope, elem, attrs) {
            elem.linedtextarea();
            var parent = elem.parent();
            var linesDiv = parent;
            function lineHighlight(line) {
                var lineHeight = elem[0].scrollHeight / (elem[0].value.match(/\n/g).length + 1);
                var jump = (line - 1) * lineHeight;
                elem.scrollTop(jump);
                elem.scroll();
                parent.find('.lines div').eq(line - 1).addClass('lineerror');
            }
            function lineClear() {
                parent.find('.lineerror').removeClass('lineerror');
            }
            scope.$watch(attrs.tsLine, function (v) {
                lineClear();
                if (v) {
                    lineHighlight(v);
                }
            });
        }
    };
});

tsafApp.directive('tsTableSort', [
    '$timeout', function ($timeout) {
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

var fmtUnits = ['', 'k', 'M', 'G', 'T', 'P', 'E'];

function nfmt(s, mult, suffix, opts) {
    opts = opts || {};
    var n = parseFloat(s);
    if (opts.round)
        n = Math.round(n);
    if (!n)
        return suffix ? '0 ' + suffix : '0';
    if (isNaN(n) || !isFinite(n))
        return '-';
    var a = Math.abs(n);
    var precision = a < 1 ? 2 : 4;
    if (a >= 1) {
        var number = Math.floor(Math.log(a) / Math.log(mult));
        a /= Math.pow(mult, Math.floor(number));
        if (fmtUnits[number]) {
            suffix = fmtUnits[number] + suffix;
        }
    }
    if (n < 0)
        a = -a;
    var r = a.toFixed(precision);
    return r + suffix;
}

tsafApp.filter('nfmt', function () {
    return function (s) {
        return nfmt(s, 1000, '', {});
    };
});

tsafApp.filter('bytes', function () {
    return function (s) {
        return nfmt(s, 1024, 'B', { round: true });
    };
});

tsafApp.filter('bits', function () {
    return function (s) {
        return nfmt(s, 1024, 'b', { round: true });
    };
});

//This is modeled after the linky function, but drops support for sanitize so we don't have to
//import an unminified angular-sanitize module
tsafApp.filter('linkq', [
    '$sanitize', function ($sanitize) {
        var QUERY_REGEXP = /((q|band)\([^)]+\))/;
        return function (text, target) {
            if (!text)
                return text;
            var raw = text;
            var html = [];
            var url;
            var i;
            var match;
            while ((match = raw.match(QUERY_REGEXP))) {
                url = '/egraph?q=' + btoa(match[0]);
                i = match.index;
                addText(raw.substr(0, i));
                addLink(url, match[0]);
                raw = raw.substring(i + match[0].length);
            }
            addText(raw);
            return $sanitize(html.join(''));
            function addText(text) {
                if (!text) {
                    return;
                }
                var el = document.createElement('div');
                el.innerText = el.textContent = text;
                html.push(el.innerHTML);
            }
            function addLink(url, text) {
                html.push('<a ');
                if (angular.isDefined(target)) {
                    html.push('target="');
                    html.push(target);
                    html.push('" ');
                }
                html.push('href="');
                html.push(url);
                html.push('">');
                addText(text);
                html.push('</a>');
            }
        };
    }]);
