interface IDashboardScope extends ITsafScope {
	error: string;
	loading: string;
}

tsafControllers.controller('DashboardCtrl', ['$scope', function($scope: IDashboardScope) {
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