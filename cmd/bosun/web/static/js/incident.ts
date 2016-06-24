interface IIncidentScope extends ng.IScope {
	error: string;
	incident: any;
	events: any;
	actions: any;
	body: any;
	shown: any;
	collapse: any;
	loadTimelinePanel: any;
	config_text: any;
	lastNonUnknownAbnormalIdx: any;
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
	$scope.loadTimelinePanel = (v: any, i: any) => {
		if (v.doneLoading && !v.error) { return; }
		v.error = null;
		v.doneLoading = false;
		if (i == $scope.lastNonUnknownAbnormalIdx) {
			v.subject = $scope.incident.Subject;
			v.body = $scope.body;
			v.doneLoading = true;
			return;
		}
		var ak = $scope.incident.AlertKey;
		var openBrack = ak.indexOf("{");
		var closeBrack = ak.indexOf("}");
		var alertName = ak.substr(0, openBrack);
		var template = ak.substring(openBrack + 1, closeBrack);
		var url = '/api/rule?' +
			'alert=' + encodeURIComponent(alertName) +
			'&from=' + encodeURIComponent(moment.utc(v.Time).format()) +
			'&template_group=' + encodeURIComponent(template);
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
			$scope.actions = data.Actions;
			$scope.events = data.Events.reverse();
			for (var i = 0; i < $scope.events.length; i++) {
				var e = $scope.events[i];
				if (e.Status != 'normal' && e.Status != 'unknown') {
					$scope.lastNonUnknownAbnormalIdx = i;
					break;
				}
			}
			$scope.body = $sce.trustAsHtml(data.Body);
		})
		.error(err => {
			$scope.error = err;
		});
}]);