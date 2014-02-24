/// <reference path="angular.d.ts" />
/// <reference path="angular-route.d.ts" />
/// <reference path="bootstrap.d.ts" />
/// <reference path="moment.d.ts" />
/// <reference path="rickshaw.d.ts" />
var tsafApp = angular.module('tsafApp', [
    'ngRoute',
    'tsafControllers',
    'mgcrea.ngStrap'
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
        }).when('/graph', {
            templateUrl: 'partials/graph.html',
            controller: 'GraphCtrl'
        }).when('/host', {
            templateUrl: 'partials/host.html',
            controller: 'HostCtrl'
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
        $scope.panel = function (status) {
            if (status == "critical") {
                return "panel-danger";
            } else if (status == "warning") {
                return "panel-warning";
            }
            return "panel-default";
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
        if (!current) {
            $location.hash('avg(q("avg:os.cpu{host=ny-devtsdb04.ds.stackexchange.com}", "5m")) > 0.5');
            return;
        }
        $scope.expr = current;
        $scope.running = current;
        $http.get('/api/expr?q=' + encodeURIComponent(current)).success(function (data) {
            $scope.result = data;
            $scope.running = '';
        }).error(function (error) {
            $scope.error = error;
            $scope.running = '';
        });
        $scope.json = function (v) {
            return JSON.stringify(v, null, '  ');
        };
        $scope.set = function () {
            $location.hash($scope.expr);
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
    function Query(qp) {
        this.aggregator = qp.aggregator || 'sum';
        this.metric = qp.metric;
        this.rate = qp.rate || false;
        this.rateOptions = qp.rateOptions || new RateOptions;
        this.ds = qp.ds || '';
        this.dstime = qp.dstime || '';
        this.tags = qp.tags || new TagSet;
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
        this.Queries = [];
    }
    return Request;
})();

tsafControllers.controller('GraphCtrl', [
    '$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        $scope.aggregators = ["sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
        $scope.dsaggregators = ["", "sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
        var search = $location.search();
        var request = search.json ? JSON.parse(search.json) : new Request;
        $scope.index = parseInt($location.hash()) || 0;
        $scope.tagvs = [];
        $scope.sorted_tagks = [];
        $scope.query_p = request.Queries;
        $scope.start = request.start;
        $scope.end = request.end;
        $scope.AddTab = function () {
            $scope.index = $scope.query_p.length;
            $scope.query_p.push(new Query({}));
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
                request.Queries.push(q);
            });
            return request;
        }
        $scope.Query = function () {
            $location.search('json', JSON.stringify(getRequest()));
            $route.reload();
        };
        request = getRequest();
        if (!request.Queries.length) {
            return;
        }
        $http.get('/api/query?' + 'json=' + encodeURIComponent(JSON.stringify(request))).success(function (data) {
            $scope.result = data;
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
        $scope.idata = {};
        var cpu_q = 'metric=os.cpu&aggregator=avg&rate=true&start=' + $scope.time + '&tags=host,' + $scope.host;
        $http.get('/api/query?' + cpu_q).success(function (data) {
            $scope.cpu = data;
            $scope.running = '';
            $scope.error = '';
        }).error(function (error) {
            $scope.error = error;
            $scope.running = '';
        });
        $http.get('/api/tagv/iface/os.net.bytes?host=' + $scope.host).success(function (data) {
            $scope.interfaces = data;
            angular.forEach($scope.interfaces, function (i) {
                var net_bytes_q = 'metric=os.net.bytes&aggregator=avg&rate=true&start=' + $scope.time + '&tags=host,' + $scope.host + ',iface,' + i + ',direction,*';
                $http.get('/api/query?' + net_bytes_q).success(function (data) {
                    $scope.idata[i] = data;
                    $scope.running = '';
                    $scope.error = '';
                }).error(function (error) {
                    $scope.error = error;
                    $scope.running = '';
                });
            });
            $scope.running = '';
            $scope.error = '';
        }).error(function (error) {
            $scope.error = error;
            $scope.running = '';
        });
        var mem_total_q = 'metric=os.mem.total&aggregator=avg&start=' + $scope.time + '&tags=host,' + $scope.host;
        $http.get('/api/query?' + mem_total_q).success(function (data) {
            $scope.mem_total = data;
            $scope.running = '';
            $scope.error = '';
        }).error(function (error) {
            $scope.error = error;
            $scope.running = '';
        });
    }]);

tsafApp.directive("tsRickshaw", function () {
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
                    renderer: 'line'
                };
                if (attrs.max) {
                    graph_options.max = attrs.max;
                }
                if (attrs.renderer) {
                    graph_options.renderer = attrs.renderer;
                }
                var graph = new Rickshaw.Graph(graph_options);
                console.log(graph);
                var x_axis = new Rickshaw.Graph.Axis.Time({
                    graph: graph,
                    timeFixture: new Rickshaw.Fixtures.Time()
                });
                var y_axis = new Rickshaw.Graph.Axis.Y({
                    graph: graph,
                    orientation: 'left',
                    tickFormat: Rickshaw.Fixtures.Number.formatKMBT,
                    element: angular.element('.y_axis', elem)[0]
                });
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
            });
        }
    };
});

tsafApp.directive("tooltip", function () {
    return {
        link: function (scope, elem, attrs) {
            angular.element(elem[0]).tooltip({ placement: "bottom" });
        }
    };
});
