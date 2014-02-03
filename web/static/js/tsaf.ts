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
		when('/items', {
			templateUrl: 'partials/items.html',
			controller: 'ItemsCtrl',
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

class Metric {
}

class Hosts {
}

interface IDashboardScope extends ng.IScope {Í
	alerts: Alert[];
}

interface IItemsScope extends ng.IScope {Í
	metrics: Metric[];
	hosts: Hosts[];
	status: string;
}

interface ITmService extends ng.IHttpService {
	getMetrics: Metric[];
	getHosts: Hosts[];
}

tsafControllers.controller('DashboardCtrl', ['$scope', '$http', function($scope: IDashboardScope, $http: ng.IHttpService) {
	$http.get('/api/alerts').success(function(data) {
		
	});
}]);

tsafControllers.controller('ItemsCtrl', ['$scope', 'TmService', function($scope: IItemsScope, TmService: ITmService){
	
	$scope.metrics;
	$scope.hosts;

	getMetrics();
	getHosts();

	function getMetrics() {
		TmService.getMetrics()
			.success(function (metrics: Metric[]) {
				$scope.metrics = metrics;
			})
			.error(function (error) {
				$scope.status = 'Unable to fetch metrics: ' + error.message;
			});
	}

	function getHosts() {
		TmService.getHosts()
			.success(function (hosts: Hosts[]) {
				$scope.hosts = hosts;
			})
			.error(function (error) {
				$scope.status = 'Unable to fetch hosts: ' + error.message;
			});
	}

}]);

tsafApp.service('TmService', ['$http', function($http: ng.IHttpService) {

	this.getMetrics = function () {
		return $http.get('/api/metric');
	};

	this.getHosts = function () {
		return $http.get('/api/tagv/host');
	};

}]);