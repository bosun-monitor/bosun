interface IHistoryScope extends IBosunScope {
	ak: string;
	alert_history: any;
	error: string;
}

bosunApp.directive('tsAlertHistory', () => {
	return {
		templateUrl: '/partials/alerthistory.html',
	};
});

bosunControllers.controller('HistoryCtrl', ['$scope', '$http', '$location', '$route', function($scope: IHistoryScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService) {
	var search = $location.search();
	var keys: any = {};
	if (angular.isArray(search.key)) {
		angular.forEach(search.key, function(v) {
			keys[v] = true;
		});
	} else {
		keys[search.key] = true;
	}
	var params = Object.keys(keys).map((v: any) => { return 'ak=' + encodeURIComponent(v); }).join('&');
	$http.get('/api/status?' + params)
		.success((data) => {
			var selected_alerts: any = {};
			angular.forEach(data, function(v, ak) {
				if (!keys[ak]) {
					return;
				}
				v.History.map((h: any) => { h.Time = moment(h.Time); });
				angular.forEach(v.History, function(h: any, i: number) {
					if (i + 1 < v.History.length) {
						h.EndTime = v.History[i + 1].Time;
					} else {
						h.EndTime = moment();
					}
				});
				selected_alerts[ak] = {
					History: v.History.reverse(),
				};
			});
			if (Object.keys(selected_alerts).length > 0) {
				$scope.alert_history = selected_alerts;
			} else {
				$scope.error = 'No Matching Alerts Found';
			}
		})
		.error(err => {
			$scope.error = err;
		});
}]);
