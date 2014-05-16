tsafApp.directive('tsAckGroup', function() {
	return {
		scope: {
			ack: '=',
			groups: '=tsAckGroup',
			schedule: '=schedule',
		},
		templateUrl: '/partials/ackgroup.html',
		link: (scope: any, elem: any, attrs: any) => {
			scope.panelClass = (status: string) => {
				switch (status) {
					case "critical": return "panel-danger";
					case "unknown": return "panel-info";
					case "warning": return "panel-warning";
					default: return "panel-default";
				}
			};
			scope.shown = {};
			scope.collapse = (i: any) => {
				scope.shown[i] = !scope.shown[i];
			};
		},
	};
});

tsafApp.directive('tsState', function() {
	return {
		templateUrl: '/partials/alertstate.html',
		link: function(scope: any, elem: any, attrs: any) {
			scope.action = (type: string) => {
				var key = encodeURIComponent(scope.name);
				return '/action?type=' + type + '&key=' + key;
			};
			scope.zws = (v: string) => {
				return v.replace(/([,{}()])/g, '$1\u200b');
			};
		},
	};
});

tsafApp.directive('tsAck', () => {
	return {
		restrict: 'E',
		templateUrl: '/partials/ack.html',
	};
});

tsafApp.directive('tsClose', () => {
	return {
		restrict: 'E',
		templateUrl: '/partials/close.html',
	};
});

tsafApp.directive('tsForget', () => {
	return {
		restrict: 'E',
		templateUrl: '/partials/forget.html',
	};
});