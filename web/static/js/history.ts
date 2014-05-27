interface IHistoryScope extends ITsafScope {
	ak: string;
	alert_history: any;
	error: string;
}

tsafControllers.controller('HistoryCtrl', ['$scope', '$http', '$location', '$route', function($scope: IHistoryScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService) {
	var search = $location.search();
	$scope.ak = search.ak;
	var status: any;
	// TODO This function is part tsAckGroup, so we need to dedupe this....
	$scope.panelClass = (status: string) => {
		switch (status) {
			case "critical": return "panel-danger";
			case "unknown": return "panel-info";
			case "warning": return "panel-warning";
			default: return "panel-default";
		}
	};
	$scope.shown = {};
	// TODO Also Duplicated
	$scope.collapse = (i: any) => {
		$scope.shown[i] = !$scope.shown[i];
	};
	$http.get('/api/alerts')
			.success((data) => {
				status = data.Status;
				$scope.error = '';
				if (!status.hasOwnProperty($scope.ak)) {
					$scope.error = 'Alert Key: ' + ak + ' not found';
					return
				}
				$scope.alert_history = status[$scope.ak].History
			})
}]);