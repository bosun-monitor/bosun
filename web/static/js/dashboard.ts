interface IDashboardScope extends IBosunScope {
	error: string;
	loading: string;
	filter: string;
	keydown: any;
}

bosunControllers.controller('DashboardCtrl', ['$scope', '$location', function($scope: IDashboardScope, $location: ng.ILocationService) {
	var search = $location.search();
	$scope.loading = 'Loading';
	$scope.error = '';
	$scope.filter = search.filter;
	$scope.refresh($scope.filter).then(() => {
			$scope.loading = '';
		}, (err: any) => {
			$scope.loading = '';
			$scope.error = 'Unable to fetch alerts: ' + err;
		});
	$scope.keydown = function($event: any) {
		if ($event.keyCode == 13) {
			$location.search('filter', $scope.filter || null);
		}
	}
}]);
