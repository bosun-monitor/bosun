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

tsafControllers.controller('DashboardCtrl', [
    '$scope', '$http', function ($scope, $http) {
        $http.get('/api/alerts').success(function (data) {
        });
    }]);
