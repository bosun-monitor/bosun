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
    '$scope', '$http', '$location', function ($scope, $http, $location) {
        var current = $location.hash();
        if (!current) {
            $location.hash('q("avg:os.cpu{host=*}", "5m") * -1');
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
        };
    }]);

tsafControllers.controller('GraphCtrl', [
    '$scope', '$http', '$location', function ($scope, $http, $location) {
        $scope.aggregators = ["sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
        $scope.dsaggregators = ["", "sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
        var search = $location.search();
        $scope.ds = search.ds || '';
        $scope.aggregator = search.aggregator || 'sum';
        $scope.rate = search.rate == 'true';
        $scope.start = search.start || '1d-ago';
        $scope.metric = search.metric;
        $scope.counter = search.counter == 'true';
        $scope.dstime = search.dstime;
        $scope.end = search.end;
        $scope.cmax = search.cmax;
        $scope.creset = search.creset;
        $scope.tagset = search.tags ? JSON.parse(search.tags) : {};
        $http.get('/api/metric').success(function (data) {
            $scope.metrics = data;
        }).error(function (error) {
            $scope.error = 'Unable to fetch metrics: ' + error;
        });
        $scope.GetTagKByMetric = function () {
            var tagset = {};
            $scope.tagvs = {};
            $http.get('/api/tagk/' + $scope.metric).success(function (data) {
                if (data instanceof Array) {
                    for (var i = 0; i < data.length; i++) {
                        tagset[data[i]] = $scope.tagset[data[i]] || '';
                        GetTagVs(data[i]);
                    }
                    $scope.tagset = tagset;
                }
            }).error(function (error) {
                $scope.error = 'Unable to fetch metrics: ' + error;
            });
        };
        function TagsAsQS(ts) {
            var qts = new Array();
            for (var key in $scope.tagset) {
                if ($scope.tagset.hasOwnProperty(key)) {
                    if ($scope.tagset[key] != "") {
                        qts.push(key);
                        qts.push($scope.tagset[key]);
                    }
                }
            }
            return qts.join();
        }
        function MakeParam(qs, k, v) {
            if (v) {
                qs.push(encodeURIComponent(k) + "=" + encodeURIComponent(v));
            }
        }
        function GetTagVs(k) {
            $http.get('/api/tagv/' + k + '/' + $scope.metric).success(function (data) {
                $scope.tagvs[k] = data;
            }).error(function (error) {
                $scope.error = 'Unable to fetch metrics: ' + error;
            });
        }
        $scope.Query = function () {
            $location.search('start', $scope.start || null);
            $location.search('end', $scope.end || null);
            $location.search('aggregator', $scope.aggregator);
            $location.search('metric', $scope.metric);
            $location.search('rate', $scope.rate.toString());
            $location.search('ds', $scope.ds || null);
            $location.search('dstime', $scope.dstime || null);
            $location.search('counter', $scope.counter.toString());
            $location.search('cmax', $scope.cmax || null);
            $location.search('creset', $scope.creset || null);
            $location.search('tags', JSON.stringify($scope.tagset));
        };
        if (!$scope.metric) {
            return;
        }
        var qs = [];
        MakeParam(qs, "start", $scope.start);
        MakeParam(qs, "end", $scope.end);
        MakeParam(qs, "aggregator", $scope.aggregator);
        MakeParam(qs, "metric", $scope.metric);
        MakeParam(qs, "rate", $scope.rate.toString());
        MakeParam(qs, "tags", TagsAsQS($scope.tagset));
        if ($scope.ds && $scope.dstime) {
            MakeParam(qs, "downsample", $scope.dstime + '-' + $scope.ds);
        }
        MakeParam(qs, "counter", $scope.counter.toString());
        MakeParam(qs, "cmax", $scope.cmax);
        MakeParam(qs, "creset", $scope.creset);
        $scope.query = qs.join('&');
        $scope.running = $scope.query;
        $http.get('/api/query?' + $scope.query).success(function (data) {
            $scope.result = data;
            $scope.running = '';
            $scope.error = '';
        }).error(function (error) {
            $scope.error = error;
            $scope.running = '';
        });
    }]);

tsafApp.directive("rickShaw", function () {
    return {
        restrict: "A",
        templateUrl: '/partials/rick_template.html',
        scope: {
            series: '=',
            height: '='
        },
        link: function (scope, elem, attrs) {
            scope.$watch('series', function (v, old_v) {
                if (v != old_v) {
                    var palette = new Rickshaw.Color.Palette();
                    angular.forEach(scope.series, function (i) {
                        if (!i.hasOwnProperty('color')) {
                            i.color = palette.color();
                        }
                    });
                    var graph = new Rickshaw.Graph({
                        element: angular.element('#rgraph')[0],
                        height: scope.height,
                        min: 'auto',
                        series: scope.series,
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
                        element: angular.element('#y_axis')[0]
                    });
                    var hoverDetail = new Rickshaw.Graph.HoverDetail({
                        graph: graph
                    });
                    var legend = new Rickshaw.Graph.Legend({
                        graph: graph,
                        element: angular.element('#legend')[0]
                    });
                    graph.render();
                }
            });
        }
    };
});
