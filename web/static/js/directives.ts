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

tsafApp.directive('tsGraph', ['$filter', function($filter: ng.IFilterService) {
	var margin = {
		top: 20,
		right: 80,
		bottom: 30,
		left: 80,
	};
	return {
		scope: {
			data: '=',
			height: '=',
		},
		link: (scope: any, elem: any, attrs: any) => {
			var svgHeight = +scope.height || 150;
			var height = svgHeight - margin.top - margin.bottom;
			var svgWidth = elem.width();
			var width = svgWidth - margin.left - margin.right;
			var xScale = d3.time.scale.utc().range([0, width]);
			var yScale = d3.scale.linear().range([height, 0]);
			var xAxis = d3.svg.axis()
				.scale(xScale)
				.orient('bottom');
			var yAxis = d3.svg.axis()
				.scale(yScale)
				.orient('left');
			var line = d3.svg.line()
				.x((d) => { return xScale(d.x * 1000); })
				.y((d) => { return yScale(d.y); });
			var svg = d3.select(elem[0])
				.append('svg')
				.attr('width', svgWidth)
				.attr('height', svgHeight)
				.append('g')
				.attr('transform', 'translate(' + margin.left + ',' + margin.top + ')');
			svg.append('defs')
				.append('clipPath')
				.attr('id', 'clip')
				.append('rect')
				.attr('width', width)
				.attr('height', height);
			var chart = svg.append('g')
				.attr('clip-path', 'url(#clip)');
			svg.append('g')
				.attr('class', 'x axis')
				.attr('transform', 'translate(0,' + height + ')');
			svg.append('g')
				.attr('class', 'y axis');
			var legend = d3.select(elem[0]).append('div');
			var color = d3.scale.category10();

			scope.$watch('data', update);
			var oldx = 0;
			function update(v: any) {
				if (!angular.isArray(v) || v.length == 0) {
					return;
				}
				var xdomain = [
					d3.min(v, (d: any) => { return d3.min(d.data, (c: any) => { return c.x; }); }) * 1000,
					d3.max(v, (d: any) => { return d3.max(d.data, (c: any) => { return c.x; }); }) * 1000,
				];
				xScale.domain(xdomain);
				yScale.domain([
					d3.min(v, (d: any) => { return d3.min(d.data, (c: any) => { return c.y; }); }),
					d3.max(v, (d: any) => { return d3.max(d.data, (c: any) => { return c.y; }); }),
				]);
				if (!oldx) {
					oldx = xdomain[1];
				} else if (oldx == xdomain[1]) {
					return;
				}
				svg.select('.x.axis')
					.transition()
					.call(xAxis);
				svg.select('.y.axis')
					.transition()
					.call(yAxis);
				var queries = chart.selectAll('.line')
					.data(v, (d) => { return d.name; });
				queries.enter()
					.append('path')
					.attr('stroke', (d: any) => { return color(d.name); })
					.attr('class', 'line');
				queries.exit()
					.remove();
				queries
					.attr('d', (d: any) => { return line(d.data); })
					.attr('transform', null)
					.transition()
					.ease('linear')
					.attr('transform', 'translate(' + (xScale(oldx) - xScale(xdomain[1])) + ')');
				oldx = xdomain[1];
				var names = legend.selectAll('.series')
					.data(v, (d) => { return d.name; });
				names.enter()
					.append('div')
					.attr('class', 'series');
				names.exit()
					.remove();
				names
					.text((d: any) => { return d.name; })
					.style('color', (d: any) => { return color(d.name); });
			};
		},
	};
}]);