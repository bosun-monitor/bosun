interface IDashboardScope extends IBosunScope {
	error: string;
	loading: string;
}

bosunControllers.controller('DashboardCtrl', ['$scope', function($scope: IDashboardScope) {
	$scope.loading = 'Loading';
	$scope.error = '';
	$scope.refresh()
		.success(() => {
			$scope.loading = '';
		})
		.error((err: any) => {
			$scope.error = 'Unable to fetch alerts: ' + err;
		});
}]);
