tsafApp.directive('tsResults', function() {
	return {
		templateUrl: '/partials/results.html',
	};
});

var timeFormat = 'YYYY-MM-DD HH:mm:ss ZZ';

tsafApp.directive("tsTime", function() {
	return {
		link: function(scope: ITsafScope, elem: any, attrs: any) {
			scope.$watch(attrs.tsTime, (v: any) => {
				var m = moment(v).utc();
				var el = document.createElement('a');
				el.innerText = m.format(timeFormat) +
				' (' +
				m.fromNow() +
				')';
				el.href = 'http://www.timeanddate.com/worldclock/converted.html?iso=';
				el.href += m.format('YYYYMMDDTHHmm');
				el.href += '&p1=0';
				angular.forEach(scope.timeanddate, (v, k) => {
					el.href += '&p' + (k + 2) + '=' + v;
				});
				elem.html(el);
			});
		},
	};
});

tsafApp.directive("tsSince", function() {
	return {
		link: function(scope: ITsafScope, elem: any, attrs: any) {
			scope.$watch(attrs.tsSince, (v: any) => {
				var m = moment(v).utc();
				elem.text(m.fromNow());
			});
		},
	};
});

tsafApp.directive("tooltip", function() {
	return {
		link: function(scope: IGraphScope, elem: any, attrs: any) {
			angular.element(elem[0]).tooltip({ placement: "bottom" });
		},
	};
});

tsafApp.directive('tsLine', () => {
	return {
		link: (scope: any, elem: any, attrs: any) => {
			elem.linedtextarea();
			var parent = elem.parent();
			var linesDiv = parent
			function lineHighlight(line: any) {
				var lineHeight = elem[0].scrollHeight / (elem[0].value.match(/\n/g).length + 1);
				var jump = (line - 1) * lineHeight;
				elem.scrollTop(jump);
				elem.scroll();
				parent.find('.lines div').eq(line - 1).addClass('lineerror');
			}
			function lineClear() {
				parent.find('.lineerror').removeClass('lineerror');
			}
			scope.$watch(attrs.tsLine, (v: any) => {
				lineClear();
				if (v) {
					lineHighlight(v);
				}
			});
		},
	};
});

interface JQuery {
	tablesorter(v: any): JQuery;
}

tsafApp.directive('tsTableSort', ['$timeout', ($timeout: ng.ITimeoutService) => {
	return {
		link: (scope: ng.IScope, elem: any, attrs: any) => {
			$timeout(() => {
				$(elem).tablesorter({
					sortList: scope.$eval(attrs.tsTableSort),
				});
			});
		},
	};
}]);

var fmtUnits = ['', 'k', 'M', 'G', 'T', 'P', 'E'];

function nfmt(s: any, mult: number, suffix: string, opts: any) {
	opts = opts || {};
	var n = parseFloat(s);
	if (opts.round) n = Math.round(n);
	if (!n) return suffix ? '0 ' + suffix : '0';
	if (isNaN(n) || !isFinite(n)) return '-';
	var a = Math.abs(n);
	var precision = a < 1 ? 2 : 4;
	if (a >= 1) {
		var number = Math.floor(Math.log(a) / Math.log(mult));
		a /= Math.pow(mult, Math.floor(number));
		if (fmtUnits[number]) {
			suffix = fmtUnits[number] + suffix;
		}
	}
	if (n < 0) a = -a;
	var r = a.toFixed(precision);
	return r + suffix;
}

tsafApp.filter('nfmt', function() {
	return function(s: any) {
		return nfmt(s, 1000, '', {});
	}
});

tsafApp.filter('bytes', function() {
	return function(s: any) {
		return nfmt(s, 1024, 'B', { round: true });
	}
});

tsafApp.filter('bits', function() {
	return function(s: any) {
		return nfmt(s, 1024, 'b', { round: true });
	}
});

//This is modeled after the linky function, but drops support for sanitize so we don't have to
//import an unminified angular-sanitize module
tsafApp.filter('linkq', ['$sanitize', function($sanitize: ng.sanitize.ISanitizeService) {
	var QUERY_REGEXP: RegExp = /((q|band)\([^)]+\))/;
	return function(text: string, target: string) {
		if (!text) return text;
		var raw = text;
		var html: string[] = [];
		var url: string;
		var i: number;
		var match: any;
		while ((match = raw.match(QUERY_REGEXP))) {
			url = '/graph?q=' + btoa(match[0]);
			i = match.index;
			addText(raw.substr(0, i));
			addLink(url, match[0]);
			raw = raw.substring(i + match[0].length);
		}
		addText(raw);
		return $sanitize(html.join(''));
		function addText(text: string) {
			if (!text) {
				return;
			}
			var el = document.createElement('div');
			el.innerText = el.textContent = text;
			html.push(el.innerHTML);
		}
		function addLink(url: string, text: string) {
			html.push('<a ');
			if (angular.isDefined(target)) {
				html.push('target="');
				html.push(target);
				html.push('" ');
			}
			html.push('href="');
			html.push(url);
			html.push('">');
			addText(text);
			html.push('</a>');
		}
	};
}]);