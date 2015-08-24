interface IDashboardScope extends IBosunScope {
	error: string;
	loading: string;
	filter: string;
	keydown: any;
}

bosunControllers.controller('DashboardCtrl', ['$scope', '$http', '$location', function($scope: IDashboardScope, $http: ng.IHttpService, $location: ng.ILocationService) {
	var search = $location.search();
	$scope.loading = 'Loading';
	$scope.error = '';
	$scope.filter = search.filter;
	if (!$scope.filter) {
		$scope.filter = readCookie("filter");
	}
	$location.search('filter', $scope.filter || null);
	reload();
	function reload() {
		$scope.refresh($scope.filter).then(() => {
				$scope.loading = '';
				$scope.error = '';
			}, (err: any) => {
				$scope.loading = '';
				$scope.error = 'Unable to fetch alerts: ' + err;
			});
	}
	$scope.keydown = function($event: any) {
		if ($event.keyCode == 13) {
			createCookie("filter", $scope.filter || "", 1000);
			$location.search('filter', $scope.filter || null);
		}
	}
}]);