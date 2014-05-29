interface IHistoryScope extends ITsafScope {
	ak: string;
	alert_history: any;
	error: string;
	shown: any;
	collapse: (i: any) => void;
}

tsafControllers.controller('HistoryCtrl', ['$scope', '$http', '$location', '$route', function($scope: IHistoryScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService) {
	var search = $location.search();
	$scope.ak = search.ak;
	var status: any;
	$scope.shown = {};
	$scope.collapse = (i: any) => {
		$scope.shown[i] = !$scope.shown[i];
	};
	function done() {
		var state = $scope.schedule.Status[$scope.ak];
		if (!state) {
			$scope.error = 'Alert Key: ' + $scope.ak + ' not found';
			return;
		}
		$scope.alert_history = state.History.slice();
		$scope.alert_history.reverse();
	}
	if ($scope.schedule) {
		done();
	} else {
		$scope.refresh(done);
	}
}]);