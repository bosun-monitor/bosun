interface IIncidentScope extends ng.IScope {
	error: string;
	incident: any;
	events: any;
	actions: any;
}

bosunControllers.controller('IncidentCtrl', ['$scope', '$http', '$location', '$route', function($scope: IIncidentScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService) {
	var search = $location.search();
	var id = search.id;
	if (!id) {
		$scope.error = "must supply incident id as query parameter"
		return
	}
	$http.get('/api/incidents/events?id='+id)
		.success((data) => {
			$scope.incident = data;
			$scope.actions = data.Actions;
			$scope.events = data.Events;
		})
		.error(err => {
			$scope.error = err;
		});
}]);