/// <reference path="angular.d.ts" />
/// <reference path="angular-route.d.ts" />
/// <reference path="google.visualization.d.ts" />
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

tsafControllers.controller('DashboardCtrl', [
    '$scope', '$http', function ($scope, $http) {
        $http.get('/api/alerts').success(function (data) {
            $scope.schedule = data;
        });
        $scope.last = function (history) {
            return history[history.length - 1];
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
    '$scope', '$http', function ($scope, $http) {
        //Might be better to get these from OpenTSDB's Aggregator API
        $scope.aggregators = ["sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
        $scope.dsaggregators = ["", "sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
        $scope.ds = "";
        $scope.aggregator = "sum";
        $scope.rate = "false";
        $scope.start = "1h-ago";
        $http.get('/api/metric').success(function (data) {
            $scope.metrics = data;
        }).error(function (error) {
            $scope.error = 'Unable to fetch metrics: ' + error;
        });
        $scope.GetTagKByMetric = function () {
            $scope.tagset = {};
            $scope.tagvs = {};
            $http.get('/api/tagk/' + $scope.metric).success(function (data) {
                if (data instanceof Array) {
                    for (var i = 0; i < data.length; i++) {
                        $scope.tagset[data[i]] = "";
                        GetTagVs(data[i]);
                    }
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
        function MakeParam(k, v) {
            if (v) {
                return encodeURIComponent(k) + "=" + encodeURIComponent(v) + "&";
            }
            return "";
        }
        function GetTagVs(k) {
            $http.get('/api/tagv/' + k + '/' + $scope.metric).success(function (data) {
                $scope.tagvs[k] = data;
            }).error(function (error) {
                $scope.error = 'Unable to fetch metrics: ' + error;
            });
        }
        $scope.MakeQuery = function () {
            var qs = "";
            qs += MakeParam("start", $scope.start);
            qs += MakeParam("end", $scope.end);
            qs += MakeParam("aggregator", $scope.aggregator);
            qs += MakeParam("metric", $scope.metric);
            qs += MakeParam("rate", $scope.rate);
            qs += MakeParam("tags", TagsAsQS($scope.tagset));
            if ($scope.ds && $scope.dstime) {
                qs += MakeParam("downsample", $scope.dstime + '-' + $scope.ds);
            }
            $scope.query = qs;
            $scope.running = $scope.query;
            $http.get('/api/query?' + $scope.query).success(function (data) {
                $scope.result = data.table;
                $scope.running = '';
            }).error(function (error) {
                $scope.error = error;
                $scope.running = '';
            });
        };
    }]);

tsafApp.directive("googleChart", function () {
    return {
        restrict: "A",
        link: function (scope, elem, attrs) {
            var chart = new google.visualization.LineChart(elem[0]);
            scope.$watch(attrs.ngModel, function (v, old_v) {
                if (v != old_v) {
                    var dt = new google.visualization.DataTable(v);
                    chart.draw(dt, null);
                }
            });
        }
    };
});
