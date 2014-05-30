tsafApp.directive('tsResults', function() {
	return {
		templateUrl: '/partials/results.html',
	};
});

var timeFormat = 'YYYY-MM-DD HH:mm:ss ZZ';

interface ITimeScope extends ITsafScope {
	noLink: string;
}

tsafApp.directive("tsTime", function() {
	return {
		link: function(scope: ITimeScope, elem: any, attrs: any) {
			scope.$watch(attrs.tsTime, (v: any) => {
				var m = moment(v).utc();
				var text = m.format(timeFormat) +
				' (' +
				m.fromNow() +
				')';
				if (attrs.noLink) {
					elem.text(m.format(timeFormat) + ' (' + m.fromNow() + ')');
				} else {
					var el = document.createElement('a');
					el.innerText = text ;
					el.href = 'http://www.timeanddate.com/worldclock/converted.html?iso=';
					el.href += m.format('YYYYMMDDTHHmm');
					el.href += '&p1=0';
					angular.forEach(scope.timeanddate, (v, k) => {
						el.href += '&p' + (k + 2) + '=' + v;
					});
					elem.html(el);
				}
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

tsafApp.directive('ahTimeLine', () => {
	//2014-05-26T21:46:37.435056942Z
	var format = d3.time.format.utc("%Y-%m-%dT%X");
	var tsdbFormat = d3.time.format.utc("%Y/%m/%d-%X");
	function parseDate(s: Moment) {
		return s.toDate();
	}
	var margin = {
		top: 20,
		right: 80,
		bottom: 30,
		left: 80,
	};
	var customTimeFormat = d3.time.format.utc.multi([
		[".%L", (d: any) => { return d.getMilliseconds(); }],
		[":%S", (d: any) => { return d.getSeconds(); }],
		["%H:%M", (d: any) => { return d.getMinutes(); }],
		["%H", (d: any) => { return d.getHours(); }],
		["%a %d", (d: any) => { return d.getDay() && d.getDate() != 1; }],
		["%b %d", (d: any) => { return d.getDate() != 1; }],
		["%B", (d: any) => { return d.getMonth(); }],
		["%Y", () => { return true; }]
	]);
	return {
		link: (scope: any, elem: any, attrs: any) => {
			var svgHeight = elem.height();
			var height = svgHeight - margin.top - margin.bottom;
			var svgWidth = elem.width();
			var width = svgWidth - margin.left - margin.right;
			var xScale = d3.time.scale.utc().range([0, width]);
			var xAxis = d3.svg.axis()
				.scale(xScale)
				.tickFormat(customTimeFormat)
				.orient('bottom');
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
			var legend = d3.select('.legend')
				.append('p')
				.text(tsdbFormat(new Date));
			scope.$watch(attrs.data, update);
			function update(v: any) {
				if (!angular.isArray(v) || v.length == 0) {
					return;
				}
				xScale.domain([
					d3.min(v, (d: any) => { return parseDate(d.Time); }),
					new Date(),
				]);
				svg.select('.x.axis')
					.transition()
					.call(xAxis);
				chart.selectAll('.bars')
					.data(v)
					.enter()
					.append('rect')
					.attr('class', (d: any) => { return d.Status; } )
					.attr('x', (d: any) => { return xScale(parseDate(d.Time)); })
					.attr('y', 0)
					.attr('height', height)
					.attr('width', (d: any, i: any) => {
						return xScale(parseDate(d.EndTime)) - xScale(parseDate(d.Time));
					})
					.on('mousemove', mousemove)
					.on('click', function(d, i) {
						scope.$apply(scope.collapse(i));
						$('html, body').scrollTop($("#panel" + i).offset().top);
					});
				function mousemove() {
					var x = xScale.invert(d3.mouse(this)[0]);
					legend
						.text(tsdbFormat(x));
				}
			};
		},
	};
});

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

tsafApp.directive('tsGraph', ['$window', 'nfmtFilter', function($window: ng.IWindowService, fmtfilter: any) {
	var margin = {
		top: 10,
		right: 10,
		bottom: 30,
		left: 80,
	};
	return {
		scope: {
			data: '=',
			height: '=',
			generator: '=',
		},
		link: (scope: any, elem: any, attrs: any) => {
			var svgHeight = +scope.height || 150;
			var height = svgHeight - margin.top - margin.bottom;
			var svgWidth: number;
			var width: number;
			var yScale = d3.scale.linear().range([height, 0]);
			var xScale: any; // todo: figure out the correct type
			var xAxis = d3.svg.axis()
				.orient('bottom');
			var yAxis = d3.svg.axis()
				.scale(yScale)
				.orient('left')
				.ticks(Math.min(10, height / 20))
				.tickFormat(fmtfilter);
			var line: any;
			switch (scope.generator) {
			case 'area':
				line = d3.svg.area();
				break;
			default:
				line = d3.svg.line();
			}
			line.y((d: any) => { return yScale(d.y); });
			line.x((d: any) => { return xScale(d.x * 1000); });
			var svg = d3.select(elem[0])
				.append('svg')
				.attr('height', svgHeight)
				.append('g')
				.attr('transform', 'translate(' + margin.left + ',' + margin.top + ')');
			var defs = svg.append('defs')
				.append('clipPath')
				.attr('id', 'clip')
				.append('rect')
				.attr('height', height);
			var chart = svg.append('g')
				.attr('pointer-events', 'all')
				.attr('clip-path', 'url(#clip)');
			svg.append('g')
				.attr('class', 'x axis')
				.attr('transform', 'translate(0,' + height + ')');
			svg.append('g')
				.attr('class', 'y axis');
			var xloc = d3.select(elem[0]).append('div');
			var legend = d3.select(elem[0]).append('div');
			var color = d3.scale.category10();
			var mousex = 0;
			var oldx = 0;
			var data: any;
			var focus = svg.append('g')
				.attr('class', 'focus');
			focus.append('line');
			function mouseover() {
				var pt = d3.mouse(this);
				mousex = pt[0];
				if (data && width) {
					drawLegend();
				}
			}
			function drawLegend() {
				var names = legend.selectAll('.series')
					.data(data, (d) => { return d.name; });
				names.enter()
					.append('div')
					.attr('class', 'series');
				names.exit()
					.remove();
				var xi = xScale.invert(mousex);
				xloc.text('Time: ' + moment(xi).utc().format());
				var t = xi.getTime() / 1000;
				names
					.text((d: any) => {
						var idx = bisect(d.data, t);
						if (idx >= d.data.length) {
							idx = d.data.length - 1;
						}
						var pt = d.data[idx];
						if (pt) {
							return d.name + ': ' + pt.y;
						}
					})
					.style('color', (d: any) => { return color(d.name); });
				var x = mousex;
				if (x > width) {
					x = 0;
				}
				focus.select('line')
					.attr('x1', x)
					.attr('x2', x)
					.attr('y1', 0)
					.attr('y2', height);
			}
			scope.$watch('data', update);
			var w = angular.element($window);
			scope.$watch(() => {
				return w.width();
			}, resize, true);
			w.bind('resize', () => {
				scope.$apply();
			});
			function resize() {
				svgWidth = elem.width();
				width = svgWidth - margin.left - margin.right;
				xScale = d3.time.scale.utc().range([0, width]);
				xAxis.scale(xScale);
				if (!mousex) {
					mousex = width + 1;
				}
				svg.attr('width', svgWidth);
				defs.attr('width', width);
				xAxis.ticks(width / 60);
				draw();
				chart.selectAll('rect.click-capture').remove();
				chart.append('rect')
					.attr('class', 'click-capture')
					.style('visibility', 'hidden')
					.attr('x', 0)
					.attr('y', 0)
					.attr('height', height)
					.attr('width', width)
					.on('mousemove', mouseover);
			}
			var oldx = 0;
			var bisect = d3.bisector((d) => { return d.x; }).right;
			function update(v: any) {
				if (!angular.isArray(v) || v.length == 0) {
					return;
				}
				data = v;
				resize();
			}
			function draw() {
				if (!data || !xScale) {
					return;
				}
				var xdomain = [
					d3.min(data, (d: any) => { return d3.min(d.data, (c: any) => { return c.x; }); }) * 1000,
					d3.max(data, (d: any) => { return d3.max(d.data, (c: any) => { return c.x; }); }) * 1000,
				];
				if (!oldx) {
					oldx = xdomain[1];
				} else if (oldx == xdomain[1]) {
					return;
				}
				xScale.domain(xdomain);
				yScale.domain([
					d3.min(data, (d: any) => { return d3.min(d.data, (c: any) => { return c.y; }); }),
					d3.max(data, (d: any) => { return d3.max(d.data, (c: any) => { return c.y; }); }),
				]);
				if (scope.generator == 'area') {
					line.y0(yScale(0));
				}
				svg.select('.x.axis')
					.transition()
					.call(xAxis);
				svg.select('.y.axis')
					.transition()
					.call(yAxis);
				var queries = chart.selectAll('.line')
					.data(data, (d) => { return d.name; });
				switch (scope.generator) {
				case 'area':
					queries.enter()
						.append('path')
						.attr('stroke', (d: any) => { return color(d.name); })
						.attr('class', 'line')
						.style('fill', (d: any) => { return color(d.name); });
					break;
				default:
					queries.enter()
						.append('path')
						.attr('stroke', (d: any) => { return color(d.name); })
						.attr('class', 'line');
				}
				queries.exit()
					.remove();
				queries
					.attr('d', (d: any) => { return line(d.data); })
					.attr('transform', null)
					.transition()
					.ease('linear')
					.attr('transform', 'translate(' + (xScale(oldx) - xScale(xdomain[1])) + ')');
				oldx = xdomain[1];
				drawLegend();
			};
		},
	};
}]);
