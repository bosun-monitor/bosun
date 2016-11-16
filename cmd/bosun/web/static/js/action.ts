
/// <reference path="expr.ts" />

interface IActionScope extends IExprScope {
	type: string;
	user: string;
	message: string;
	notify: boolean;
	keys: string[];
	submit: () => void;
	validateMsg: () => void;
	msgValid: boolean;
}

bosunControllers.controller('ActionCtrl', ['$scope', '$http', '$location', '$route', function ($scope: IActionScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService) {
	var search = $location.search();
	$scope.type = search.type;
	$scope.notify = true;
	$scope.msgValid = true;
	$scope.message = "";
	$scope.validateMsg = () => {
		$scope.msgValid = (!$scope.notify) || ($scope.message != "");
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
		if (!$scope.msgValid || ($scope.user == "")) {
			return;
		}
		var data = {
			Type: $scope.type,
			Message: $scope.message,
			Keys: $scope.keys,
			Notify: $scope.notify,
		};
		$http.post('/api/action', data)
			.success((data) => {
				$location.url('/');
			})
			.error((error) => {
				alert(error);
			});
	};
}]);