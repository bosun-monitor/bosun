/// <reference path="0-bosun.ts" />

interface IItemsScope extends ng.IScope {
	metrics: string[];
	hosts: string[];
	filterMetrics: string;
	filterHosts: string;
	status: string;
}

bosunControllers.controller('ItemsCtrl', ['$scope', '$http', function($scope: IItemsScope, $http: ng.IHttpService) {
	$http.get('/api/metric')
		.then(function(data: string[]) {
			$scope.metrics = data;
		},function(error) {
			$scope.status = 'Unable to fetch metrics: ' + error;
		});
	$http.get('/api/tagv/host?since=default')
		.then(function(data: string[]) {
			$scope.hosts = data;
		},function(error) {
			$scope.status = 'Unable to fetch hosts: ' + error;
		});
}]);
