/// <reference path="angular.d.ts" />
/// <reference path="angular-route.d.ts" />
var tsafApp = angular.module('tsafApp', [
    'ngRoute',
    'tsafControllers'
]);

tsafApp.config([
    '$routeProvider', function ($routeProvider) {
        $routeProvider.when('/', {
            templateUrl: 'partials/dashboard.html',
            controller: 'DashboardCtrl'
        }).when('/items', {
            templateUrl: 'partials/items.html',
            controller: 'ItemsCtrl'
        }).otherwise({
            redirectTo: '/'
        });
    }]);

var tsafControllers = angular.module('tsafControllers', []);

var Alert = (function () {
    function Alert() {
    }
    return Alert;
})();

var Schedule = (function () {
    function Schedule() {
    }
    return Schedule;
})();

var Metric = (function () {
    function Metric() {
    }
    return Metric;
})();

var Hosts = (function () {
    function Hosts() {
    }
    return Hosts;
})();

tsafControllers.controller('DashboardCtrl', [
    '$scope', '$http', function ($scope, $http) {
        $http.get('/api/alerts').success(function (data) {
        });
    }]);

tsafControllers.controller('ItemsCtrl', [
    '$scope', 'TmService', function ($scope, TmService) {
        $scope.metrics;
        $scope.hosts;

        getMetrics();
        getHosts();

        function getMetrics() {
            TmService.getMetrics().success(function (metrics) {
                $scope.metrics = metrics;
            }).error(function (error) {
                $scope.status = 'Unable to fetch metrics: ' + error.message;
            });
        }

        function getHosts() {
            TmService.getHosts().success(function (hosts) {
                $scope.hosts = hosts;
            }).error(function (error) {
                $scope.status = 'Unable to fetch hosts: ' + error.message;
            });
        }
    }]);

tsafApp.service('TmService', [
    '$http', function ($http) {
        this.getMetrics = function () {
            return $http.get('/api/metric');
        };

        this.getHosts = function () {
            return $http.get('/api/tagv/host');
        };
    }]);
