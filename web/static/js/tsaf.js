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
            templateUrl: 'partials/dashboard.html',
            controller: 'DashboardCtrl'
        }).when('/items', {
            templateUrl: 'partials/items.html',
            controller: 'ItemsCtrl'
        }).when('/expr', {
            templateUrl: 'partials/expr.html',
            controller: 'ExprCtrl'
        }).when('/egraph', {
            templateUrl: 'partials/egraph.html',
            controller: 'EGraphCtrl'
        }).when('/graph', {
            templateUrl: 'partials/graph.html',
            controller: 'GraphCtrl'
        }).when('/host', {
            templateUrl: 'partials/host.html',
            controller: 'HostCtrl'
        }).when('/rule', {
            templateUrl: 'partials/rule.html',
            controller: 'RuleCtrl'
        }).otherwise({
            redirectTo: '/'
        });
    }]);

var tsafControllers = angular.module('tsafControllers', []);

tsafControllers.controller('TsafCtrl', [
    '$scope', '$route', function ($scope, $route) {
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
    }]);

tsafControllers.controller('DashboardCtrl', [
    '$scope', '$http', function ($scope, $http) {
        $http.get('/api/alerts').success(function (data) {
            angular.forEach(data.Status, function (v, k) {
                v.Touched = moment(v.Touched).utc();
                angular.forEach(v.History, function (v, k) {
                    v.Time = moment(v.Time).utc();
                });
                v.last = v.History[v.History.length - 1];
            });
            $scope.schedule = data;
        });
        $scope.collapse = function (i) {
            $('#collapse' + i).collapse('toggle');
        };
        $scope.panelClass = function (status) {
            switch (status) {
                case 3:
                    return "panel-danger";
                    break;
                case 2:
                    return "panel-warning";
                    break;
                default:
                    return "panel-default";
                    break;
            }
        };
        $scope.statusString = function (status) {
            switch (status) {
                case 3:
                    return "critical";
                    break;
                case 2:
                    return "warning";
                    break;
                case 1:
                    return "normal";
                    break;
                default:
                    return "unknown";
                    break;
            }
        };
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
            $location.hash(btoa('avg(q("avg:rate:os.cpu{host=ny-devtsdb04}", "5m")) > 80'));
            return;
        }
        $scope.expr = current;
        $scope.running = current;
        $http.get('/api/expr?q=' + encodeURIComponent(current)).success(function (data) {
            $scope.result = data.Results;
            $scope.queries = data.Queries;
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
        $scope.bytes = search.bytes;
        $scope.bytes = !!$scope.bytes;
        $scope.renderers = ['area', 'bar', 'line', 'scatterplot'];
        $scope.render = search.render || 'scatterplot';
        if (!current) {
            $location.search('q', btoa('q("avg:rate:os.cpu{host=ny-devtsdb04}", "5m")'));
            return;
        }
        $scope.expr = current;
        $scope.running = current;
        var width = $('.chart').width();
        $http.get('/api/egraph?q=' + encodeURIComponent(current) + '&autods=' + width).success(function (data) {
            $scope.result = data;
            $scope.running = '';
        }).error(function (error) {
            $scope.error = error;
            $scope.running = '';
        });
        $scope.set = function () {
            $location.search('q', btoa($scope.expr));
            $location.search('render', $scope.render);
            $location.search('bytes', $scope.bytes);
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
        this.rateOptions = q && q.rateOptions || new RateOptions;
        this.ds = q && q.ds || '';
        this.dstime = q && q.dstime || '';
        this.tags = q && q.tags || new TagSet;
        this.setDs();
    }
    Query.prototype.setDs = function () {
        if (this.dstime && this.ds) {
            this.downsample = this.dstime + '-' + this.ds;
        } else {
            this.downsample = '';
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
                switch (typeof v) {
                    case "string":
                        if (!v) {
                            delete _this.queries[i][k];
                        }
                        break;
                    case "boolean":
                        if (!v) {
                            delete _this.queries[i][k];
                        }
                        break;
                    case "object":
                        if (Object.keys(v).length == 0) {
                            delete _this.queries[i][k];
                        }
                        break;
                }
            });
        }
    };
    return Request;
})();

tsafControllers.controller('GraphCtrl', [
    '$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        $scope.aggregators = ["sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
        $scope.dsaggregators = ["", "sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
        var search = $location.search();
        var j = search.json;
        if (search.b64) {
            j = atob(search.b64);
        }
        var request = j ? JSON.parse(j) : new Request;
        $scope.index = parseInt($location.hash()) || 0;
        $scope.tagvs = [];
        $scope.sorted_tagks = [];
        $scope.query_p = request.queries;
        $scope.start = request.start;
        $scope.end = request.end;
        $scope.autods = search.autods;
        if (typeof $scope.autods == 'undefined') {
            $scope.autods = true;
        }
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
                    if (data instanceof Array) {
                        var tags = {};
                        for (var i = 0; i < data.length; i++) {
                            tags[data[i]] = $scope.query_p[index].tags[data[i]] || '';
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
                    }
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
            $location.search('b64', btoa(JSON.stringify(getRequest())));
            $location.search('autods', $scope.autods);
            $route.reload();
        };
        request = getRequest();
        if (!request.queries.length) {
            return;
        }
        var autods = $scope.autods ? autods = '&autods=' + $('.chart').width() : '';
        request.prune();
        $http.get('/api/graph?' + 'b64=' + btoa(JSON.stringify(request)) + autods).success(function (data) {
            $scope.result = data;
            $scope.url = $location.absUrl();
            $scope.running = '';
            $scope.error = '';
        }).error(function (error) {
            $scope.error = error;
            $scope.running = '';
        });
    }]);

tsafControllers.controller('HostCtrl', [
    '$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        $scope.host = ($location.search()).host;
        $scope.time = ($location.search()).time;
        $scope.idata = [];
        $scope.fsdata = [];
        $scope.fs_current = [];
        var cpu_r = new Request();
        cpu_r.start = $scope.time;
        cpu_r.queries = [
            new Query({
                metric: "os.cpu",
                rate: true,
                tags: { host: $scope.host }
            })
        ];
        var width = $('.chart').width();
        $http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(cpu_r)) + '&autods=' + width).success(function (data) {
            data[0].name = 'Percent Used';
            $scope.cpu = data;
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
                    angular.forEach(data, function (d) {
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
                    $scope.idata[$scope.interfaces.indexOf(i)] = { name: i, data: data };
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
                    data[1].name = "Used";
                    $scope.fsdata[$scope.fs.indexOf(i)] = { name: i, data: [data[1]] };
                    var total = Math.max.apply(null, data[0].data.map(function (d) {
                        return d.y;
                    }));
                    var c_val = data[1].data.slice(-1)[0].y;
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
            data[1].name = "Used";
            $scope.mem_total = Math.max.apply(null, data[0].data.map(function (d) {
                return d.y;
            }));
            $scope.mem = [data[1]];
        });
    }]);

tsafControllers.controller('RuleCtrl', [
    '$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        var current = $location.hash();
        try  {
            current = atob(current);
        } catch (e) {
            current = '';
        }
        if (!current) {
            var def = '$t = "5m"\n' + 'crit = avg(q("avg:os.cpu", $t, "")) > 10';
            $location.hash(btoa(def));
            return;
        }
        $scope.expr = current;
        $scope.running = current;
        $http.get('/api/rule?q=' + encodeURIComponent(current)).success(function (data) {
            $scope.result = data.Results;
            $scope.queries = data.Queries;
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

tsafApp.directive("tsRickshaw", [
    '$filter', function ($filter) {
        return {
            templateUrl: '/partials/rickshaw.html',
            link: function (scope, elem, attrs) {
                scope.$watch(attrs.tsRickshaw, function (v) {
                    if (!v) {
                        return;
                    }
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
                                label.innerHTML = d.name + ": " + d.formattedYValue;
                                if (attrs.bytes == "true") {
                                    label.innerHTML = d.name + ": " + $filter('bytes')(d.formattedYValue);
                                }
                                if (attrs.bits) {
                                    label.innerHTML = d.name + ": " + $filter('bits')(d.formattedYValue);
                                }
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

tsafApp.filter('bytes', function () {
    return function (bytes, precision) {
        if (!bytes) {
            return '0 B';
        }
        ;
        if (isNaN(parseFloat(bytes)) || !isFinite(bytes))
            return '-';
        if (typeof precision == 'undefined')
            precision = 1;
        var units = ['B', 'kB', 'MB', 'GB', 'TB', 'PB'], number = Math.floor(Math.log(bytes) / Math.log(1024));
        return (bytes / Math.pow(1024, Math.floor(number))).toFixed(precision) + ' ' + units[number];
    };
});

tsafApp.filter('bits', function () {
    return function (b, precision) {
        if (!b) {
            return '0 b';
        }
        ;
        if (b < 0) {
            b = -b;
        }
        if (isNaN(parseFloat(b)) || !isFinite(b))
            return '-';
        if (typeof precision == 'undefined')
            precision = 1;
        var units = ['b', 'kb', 'Mb', 'Gb', 'Tb', 'Pb'], number = Math.floor(Math.log(b) / Math.log(1024));
        return (b / Math.pow(1024, Math.floor(number))).toFixed(precision) + ' ' + units[number];
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
                html.push(text);
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
