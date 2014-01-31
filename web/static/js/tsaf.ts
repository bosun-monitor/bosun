/// <reference path="angular.d.ts" />
/// <reference path="angular-route.d.ts" />

var tsafApp = angular.module('tsafApp', [
	'ngRoute',
	'tsafControllers',
]);

tsafApp.config(['$routeProvider', function($routeProvider: ng.route.IRouteProvider) {
	$routeProvider.
		when('/', {
			templateUrl: 'partials/dashboard.html',
			controller: 'DashboardCtrl',
		}).
		otherwise({
			redirectTo: '/',
		});
}]);

var tsafControllers = angular.module('tsafControllers', []);

class Alert {
	
}

class Schedule {
}

interface IDashboardScope extends ng.IScope {
	alerts: Alert[];
}

tsafControllers.controller('DashboardCtrl', ['$scope', '$http', function($scope: IDashboardScope, $http: ng.IHttpService) {
	$http.get('/api/alerts').success(function(data) {
		
	});
}]);