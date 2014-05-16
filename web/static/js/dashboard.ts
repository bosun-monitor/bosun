interface IDashboardScope extends ITsafScope {
}

tsafControllers.controller('DashboardCtrl', ['$scope', function($scope: IDashboardScope) {
	$scope.refresh();
}]);