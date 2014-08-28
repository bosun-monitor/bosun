interface IDashboardScope extends IBosunScope {
	error: string;
	loading: string;
}

bosunControllers.controller('DashboardCtrl', ['$scope', function($scope: IDashboardScope) {
	$scope.loading = 'Loading';
	$scope.error = '';
	$scope.refresh().then(() => {
			$scope.loading = '';
		}, (err: any) => {
			$scope.loading = '';
			$scope.error = 'Unable to fetch alerts: ' + err;
		});
}]);
