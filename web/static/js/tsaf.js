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

var QueryParams = (function () {
    function QueryParams() {
        this.rateOptions = new RateOptions;
    }
    return QueryParams;
})();

var Query = (function () {
    function Query(qp) {
        this.aggregator = qp.aggregator;
        this.metric = qp.metric;
        this.rate = qp.rate;
        this.rateOptions = qp.rateOptions;
        if (qp.dstime && qp.ds) {
            this.Downsample = qp.dstime + '-' + qp.ds;
        }
        if (qp.tags) {
            var ts = new TagSet;
            angular.forEach(qp.tags, function (v, k) {
                if (v) {
                    ts[k] = v;
                }
            });
        }
        this.Tags = ts;
    }
    return Query;
})();

var Request = (function () {
    function Request() {
    }
    return Request;
})();

tsafControllers.controller('GraphCtrl', [
    '$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        $scope.aggregators = ["sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
        $scope.dsaggregators = ["", "sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
        var search = $location.search();
        $scope.query_p = [];
        $scope.query_p[0] = new QueryParams;
        $scope.tagvs = [];
        $scope.request = search.json ? JSON.parse(search.json) : new Request;
        $scope.start = $scope.request.start || '1d-ago';
        $scope.end = $scope.request.end;
        angular.forEach($scope.request.queries, function (q) {
            $scope.query_p[0].metric = q.metric;
            $scope.query_p[0].ds = q.ds;
            $scope.query_p[0].dstime = q.dstime;
            $scope.query_p[0].aggregator = q.aggregator || 'sum';
            $scope.query_p[0].rate = q.rate == true;
            if (q.RateOptions) {
                $scope.query_p[0].rateOptions.counter = q.rateOptions.counter == true;
                $scope.query_p[0].rateOptions.counterMax = q.rateOptions.counterMax;
                $scope.query_p[0].rateOptions.resetValue = q.rateOptions.resetValue;
            }
            $scope.query_p[0].tags = q.Tags || new TagSet;
        });

        //
        $http.get('/api/metric').success(function (data) {
            $scope.metrics = data;
        }).error(function (error) {
            $scope.error = 'Unable to fetch metrics: ' + error;
        });
        $scope.GetTagKByMetric = function () {
            var tags = {};
            $scope.tagvs[0] = new TagV;
            $http.get('/api/tagk/' + $scope.query_p[0].metric).success(function (data) {
                if (data instanceof Array) {
                    for (var i = 0; i < data.length; i++) {
                        tags[data[i]] = $scope.query_p[0].tags[data[i]] || '';
                        GetTagVs(data[i]);
                    }
                    $scope.query_p[0].tags = tags;
                }
            }).error(function (error) {
                $scope.error = 'Unable to fetch metrics: ' + error;
            });
        };
        function GetTagVs(k) {
            $http.get('/api/tagv/' + k + '/' + $scope.query_p[0].metric).success(function (data) {
                $scope.tagvs[0][k] = data;
            }).error(function (error) {
                $scope.error = 'Unable to fetch metrics: ' + error;
            });
        }
        $scope.Query = function () {
            $scope.queries = [];
            angular.forEach($scope.query_p, function (p) {
                var query = new Query(p);
                $scope.queries.push(query);
            });
            $scope.request = {
                start: $scope.start,
                end: $scope.end,
                queries: $scope.queries
            };
            $location.search('json', JSON.stringify($scope.request));
            $route.reload();
        };
        if (!$scope.query_p[0].metric) {
            console.log("metric not defined");
            return;
        }
        $http.get('/api/query?' + 'json=' + encodeURIComponent(JSON.stringify($scope.request))).success(function (data) {
            $scope.result = data;
            console.log(data);
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
                var graph = new Rickshaw.Graph({
                    element: rgraph[0],
                    height: rgraph.height(),
                    min: 'auto',
                    series: v,
                    renderer: 'line'
                });
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
        restrict: 'A',
        link: function (scope, elem, attrs) {
            angular.element(elem[0]).tooltip({ placement: "bottom" });
        }
    };
});
