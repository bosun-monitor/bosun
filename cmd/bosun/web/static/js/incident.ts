interface IIncidentScope extends ng.IScope {
	error: string;
	incident: IncidentState;
	events: any;
	actions: any;
	body: any;
	shown: any;
	collapse: any;
	loadTimelinePanel: any;
	config_text: any;
	lastNonUnknownAbnormalIdx: any;
	state: any;
	action: any;
	configLink: string;
}

bosunControllers.controller('IncidentCtrl', ['$scope', '$http', '$location', '$route', '$sce', function ($scope: IIncidentScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService, $sce: ng.ISCEService) {
	var search = $location.search();
	var id = search.id;
	if (!id) {
		$scope.error = "must supply incident id as query parameter"
		return
	}
	$http.get('/api/config')
		.success((data: any) => {
			$scope.config_text = data;
		});
	$scope.action = (type: string) => {
		var key = encodeURIComponent($scope.state.AlertKey);
		return '/action?type=' + type + '&key=' + key;
	};
	$scope.loadTimelinePanel = (v: any, i: any) => {
		if (v.doneLoading && !v.error) { return; }
		v.error = null;
		v.doneLoading = false;
		if (i == $scope.lastNonUnknownAbnormalIdx && $scope.body) {
			v.subject = $scope.incident.Subject;
			v.body = $scope.body;
			v.doneLoading = true;
			return;
		}
		var ak = $scope.incident.AlertKey;
		var url = ruleUrl(ak, moment(v.Time));
		$http.post(url, $scope.config_text)
			.success((data: any) => {
				v.subject = data.Subject;
				v.body = $sce.trustAsHtml(data.Body);
			})
			.error((error) => {
				v.error = error;
			})
			.finally(() => {
				v.doneLoading = true;
			});
	};
	$scope.shown = {};
	$scope.collapse = (i: any, v: any) => {
		$scope.shown[i] = !$scope.shown[i];
		if ($scope.loadTimelinePanel && $scope.shown[i]) {
			$scope.loadTimelinePanel(v, i);
		}
	};
	$http.get('/api/incidents/events?id=' + id)
		.success((data: any) => {
			$scope.incident = data;
			$scope.state = $scope.incident;
			$scope.actions = data.Actions;
			$scope.body = $sce.trustAsHtml(data.Body);
			$scope.events = data.Events.reverse();
			$scope.configLink = configUrl($scope.incident.AlertKey, moment.unix($scope.incident.LastAbnormalTime));
			for (var i = 0; i < $scope.events.length; i++) {
				var e = $scope.events[i];
				if (e.Status != 'normal' && e.Status != 'unknown' && $scope.body) {
					$scope.lastNonUnknownAbnormalIdx = i;
					$scope.collapse(i, e); // Expand the panel of the current body
					break;
				}
			}
			$scope.collapse;
		})
		.error(err => {
			$scope.error = err;
		});
}]);