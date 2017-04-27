
/// <reference path="expr.ts" />

interface IActionScope extends IBosunScope {
	type: string;
	user: string;
	message: string;
	notify: boolean;
	keys: string[];
	submit: () => void;
	validateMsg: () => void;
	msgValid: boolean;
	activeIncidents: boolean;
	duration: string;
	durationValid: boolean;
}

bosunControllers.controller('ActionCtrl', ['$scope', '$http', '$location', '$route', function ($scope: IActionScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService) {
	var search = $location.search();
	$scope.type = search.type;
	$scope.activeIncidents = search.active == "true";
	$scope.notify = true;
	$scope.msgValid = true;
	$scope.message = "";
	$scope.validateMsg = () => {
		$scope.msgValid = (!$scope.notify) || ($scope.message != "");
	}
	$scope.durationValid = true;
	$scope.validateDuration = () => {
		$scope.durationValid = $scope.duration == "" || parseDuration($scope.duration).asMilliseconds() != 0;
	}

	if (search.key) {
		var keys = search.key;
		if (!angular.isArray(search.key)) {
			keys = [search.key];
		}
		$location.search('key', null);
		$scope.setKey('action-keys', keys);
	} else {
		$scope.keys = $scope.getKey('action-keys');
	}
	$scope.submit = () => {
		$scope.validateMsg();
		$scope.validateDuration();
		if (!$scope.msgValid || ($scope.user == "") || !$scope.durationValid) {
			return;
		}
		var data = {
			Type: $scope.type,
			Message: $scope.message,
			Keys: $scope.keys,
			Notify: $scope.notify,
		};
		if ($scope.duration != "") {
			data['Time'] = moment.utc().add(parseDuration($scope.duration));
		}
		$http.post('/api/action', data)
			.success((data) => {
				$location.url('/');
			})
			.error((error) => {
				alert(error);
			});
	};
}]);