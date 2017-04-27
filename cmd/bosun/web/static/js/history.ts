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
	$http.get('/api/status?' + params + "&all=1")
		.then((data) => {
			console.log(data);
			var selected_alerts: any = {};
			angular.forEach(data, function(v, ak) {
				if (!keys[ak]) {
					return;
				}
				v.Events.map((h: any) => { h.Time = moment.utc(h.Time); });
				angular.forEach(v.Events, function(h: any, i: number) {
					if (i + 1 < v.Events.length) {
						h.EndTime = v.Events[i + 1].Time;
					} else {
						h.EndTime = moment.utc();
					}
				});
				selected_alerts[ak] = {
					History: v.Events.reverse(),
				};
			});
			if (Object.keys(selected_alerts).length > 0) {
				$scope.alert_history = selected_alerts;
			} else {
				$scope.error = 'No Matching Alerts Found';
			}
		},err => {
			$scope.error = err;
		});
}]);
