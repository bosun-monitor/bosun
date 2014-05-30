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
	var selected_alerts = [];
	function done() {
		var status = $scope.schedule.Status;
		// if (!status) {
		// 	$scope.error = 'Alert Key: ' + $scope.ak + ' not found';
		// 	return;
		// }
		angular.forEach(status, function(v, ak) {
			angular.forEach(v.History, function(h: any, i: number) {
				if ( i+1 < v.History.length) {
					h.EndTime = v.History[i+1].Time;
				} else {
					h.EndTime = moment.utc();
				}
			});
			v.History.reverse();
			var dict = {};
			dict['Name'] = ak;
			dict['History'] = v.History;
			selected_alerts.push(dict);
		});
		$scope.alert_history = selected_alerts.slice(0,30);
	}
	if ($scope.schedule) {
		done();
	} else {
		$scope.refresh(done);
	}
}]);